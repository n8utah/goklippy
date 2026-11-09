// Command mcu-motor-test drives real steppers through the same Session code the API
// uses and reports timing measured from the MCU's own step position.
//
//	go run ./cmd/mcu-motor-test --port COM4 --test latency
//	go run ./cmd/mcu-motor-test --port COM4 --test jog --oid 0
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goklippy/backend/klipperconfig"
	"goklippy/backend/mcu"
	"goklippy/backend/mcu/cfgtranslate"
)

type harness struct {
	s     *mcu.Session
	oid   int
	speed float64
}

var (
	holdLimit = flag.Duration("hold-limit", 10*time.Minute, "hold test: stop after this long")
	stopFile  = flag.String("stop-file", "", "hold test: stop when this file appears")
	oidList   = flag.String("oids", "0,2,5", "group tests: stepper oids")
	segSteps  = flag.Int("seg-steps", 400, "routine tests: steps per segment")
)

func main() {
	port := flag.String("port", "COM4", "serial port")
	baud := flag.Int("baud", 250000, "serial baud rate")
	config := flag.String("config", "testdata/stepper-x-only.cfg", "Klipper config to connect with")
	test := flag.String("test", "latency", "latency|jog|distance|startstop|routine")
	oid := flag.Int("oid", 0, "stepper oid")
	speed := flag.Float64("speed", 400, "speed in steps/s")
	cycles := flag.Int("cycles", 10, "start/stop cycles")
	telemetry := flag.Bool("telemetry", true, "run the telemetry loop as the UI does")
	flag.Parse()

	if err := run(*port, *baud, *config, *test, *oid, *speed, *cycles, *telemetry); err != nil {
		slog.Error("motor test failed", "error", err)
		os.Exit(1)
	}
}

func run(port string, baud int, config, test string, oid int, speed float64, cycles int, telemetry bool) error {
	f, err := klipperconfig.LoadFile(config)
	if err != nil {
		return err
	}
	build, err := cfgtranslate.Build(f)
	if err != nil {
		return err
	}

	start := time.Now()
	session, err := mcu.DefaultManager().Connect(port, baud, 5*time.Second, build)
	if err != nil {
		return err
	}
	defer func() { _ = mcu.DefaultManager().Disconnect(session.ID) }()
	fmt.Printf("connected in %s\n", time.Since(start).Round(time.Millisecond))

	if telemetry {
		updates := session.SubscribeTelemetry()
		go func() {
			for range updates {
			}
		}()
	}

	h := &harness{s: session, oid: oid, speed: speed}
	h.status("start")
	defer h.status("end")

	switch test {
	case "latency":
		return h.latency()
	case "jog":
		return h.jog()
	case "distance":
		return h.distance()
	case "startstop":
		return h.startStop(cycles)
	case "routine":
		return h.routine()
	case "hold":
		return h.hold()
	case "reset":
		return h.reset()
	case "group":
		return h.group(cycles)
	case "routine-group":
		return h.routineGroup()
	case "routine-hold":
		return h.routineHold()
	}
	return fmt.Errorf("unknown test %q", test)
}

func (h *harness) status(label string) {
	st, err := h.s.Conn.GetConfig()
	if err != nil {
		fmt.Printf("[%s] get_config error: %v\n", label, err)
		return
	}
	sent, acked := h.s.Conn.SeqState()
	fmt.Printf("[%s] is_config=%v is_shutdown=%v move_count=%d host_seq=%d mcu_expected_seq=%d\n",
		label, st.IsConfig, st.IsShutdown, st.MoveCount, sent, acked)
}

func report(name string, samples []time.Duration) {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	var total time.Duration
	for _, d := range samples {
		total += d
	}
	fmt.Printf("%-34s min=%-8s avg=%-8s max=%s\n", name,
		samples[0].Round(100*time.Microsecond),
		(total / time.Duration(len(samples))).Round(100*time.Microsecond),
		samples[len(samples)-1].Round(100*time.Microsecond))
}

func timed(n int, fn func() error) ([]time.Duration, error) {
	out := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		t := time.Now()
		if err := fn(); err != nil {
			return nil, err
		}
		out = append(out, time.Since(t))
	}
	return out, nil
}

// latency measures the serial layer alone; it sends nothing that moves a motor.
func (h *harness) latency() error {
	conn := h.s.Conn
	cases := []struct {
		name string
		fn   func() error
	}{
		{"get_clock round trip", func() error { _, err := conn.MCUClock(); return err }},
		{"get_config round trip", func() error { _, err := conn.GetConfig(); return err }},
		{"SendCommand x1", func() error { return conn.SendCommand("get_uptime") }},
		{"SendCommands x8", func() error {
			// Only sets the direction pin: no reply and no motion, like real move traffic.
			cmd := fmt.Sprintf("set_next_step_dir oid=%d dir=0", h.oid)
			return conn.SendCommands([]string{cmd, cmd, cmd, cmd, cmd, cmd, cmd, cmd})
		}},
	}
	for _, c := range cases {
		samples, err := timed(20, c.fn)
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		report(c.name, samples)
	}
	return nil
}

func (h *harness) position() int64 {
	pos, err := h.s.StepperPosition(h.oid)
	if err != nil {
		fmt.Printf("  position error: %v\n", err)
	}
	return pos
}

// waitForMotion polls the MCU step position until it changes, returning how long it took.
func (h *harness) waitForMotion(from int64, limit time.Duration) (time.Duration, bool) {
	t := time.Now()
	for time.Since(t) < limit {
		if h.position() != from {
			return time.Since(t), true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return limit, false
}

// waitForRest polls until the position stops changing, returning when it last moved.
func (h *harness) waitForRest(limit time.Duration) time.Duration {
	t := time.Now()
	last := h.position()
	lastMove := time.Duration(0)
	for time.Since(t) < limit {
		time.Sleep(5 * time.Millisecond)
		if p := h.position(); p != last {
			last = p
			lastMove = time.Since(t)
		} else if time.Since(t)-lastMove > 300*time.Millisecond {
			break
		}
	}
	return lastMove
}

func (h *harness) jog() error {
	from := h.position()
	t := time.Now()
	if err := h.s.StepperJog(h.oid, true, h.speed, 0); err != nil {
		return err
	}
	callDur := time.Since(t)
	wait, moved := h.waitForMotion(from, 3*time.Second)
	fmt.Printf("jog call=%s first_step_after_call_returned=%s moved=%v\n", callDur.Round(time.Millisecond), wait.Round(time.Millisecond), moved)

	time.Sleep(3 * time.Second)
	before := h.position()
	t = time.Now()
	if err := h.s.StepperStop(h.oid); err != nil {
		return err
	}
	callDur = time.Since(t)
	rest := h.waitForRest(2 * time.Second)
	after := h.position()
	fmt.Printf("stop call=%s last_step_after_call_returned=%s steps_while_stopping=%d total_steps=%d\n",
		callDur.Round(time.Millisecond), rest.Round(time.Millisecond), after-before, after-from)
	return nil
}

// hold jogs continuously so someone can watch the motor. It always ends with a stop
// command: the MCU keeps stepping its queue if the host just goes away.
func (h *harness) hold() error {
	from := h.position()
	if err := h.s.StepperJog(h.oid, true, h.speed, 0); err != nil {
		return err
	}
	defer func() {
		if err := h.s.StepperStop(h.oid); err != nil {
			fmt.Printf("stop error: %v\n", err)
		}
		fmt.Println("stopped")
	}()

	start := time.Now()
	for time.Since(start) < *holdLimit {
		if *stopFile != "" {
			if _, err := os.Stat(*stopFile); err == nil {
				return nil
			}
		}
		time.Sleep(time.Second)
		fmt.Printf("holding t=%s steps=%d\n", time.Since(start).Round(time.Second), h.position()-from)
	}
	return nil
}

// reset proves motion works before and after a board reset on one connection.
func (h *harness) reset() error {
	if err := h.distance(); err != nil {
		return err
	}
	t := time.Now()
	if err := h.s.Reset(); err != nil {
		return err
	}
	fmt.Printf("reset took %s\n", time.Since(t).Round(time.Millisecond))
	h.status("after reset")
	return h.distance()
}

func (h *harness) distance() error {
	const steps = 800
	from := h.position()
	t := time.Now()
	if err := h.s.StepperJog(h.oid, true, h.speed, steps); err != nil {
		return err
	}
	callDur := time.Since(t)
	wait, moved := h.waitForMotion(from, 3*time.Second)
	rest := h.waitForRest(time.Duration(float64(steps)/h.speed*float64(time.Second)) + 2*time.Second)
	fmt.Printf("distance call=%s first_step=%s moved=%v finished_after=%s steps=%d (want %d)\n",
		callDur.Round(time.Millisecond), wait.Round(time.Millisecond), moved, rest.Round(time.Millisecond), h.position()-from, steps)
	return nil
}

func (h *harness) startStop(cycles int) error {
	failed := 0
	var starts, stops []time.Duration
	for i := 0; i < cycles; i++ {
		from := h.position()
		t := time.Now()
		if err := h.s.StepperJog(h.oid, i%2 == 0, h.speed, 0); err != nil {
			return fmt.Errorf("cycle %d jog: %w", i, err)
		}
		_, moved := h.waitForMotion(from, 2*time.Second)
		starts = append(starts, time.Since(t))
		if !moved {
			failed++
			fmt.Printf("cycle %d: NO MOTION\n", i)
		}
		time.Sleep(400 * time.Millisecond)
		t = time.Now()
		if err := h.s.StepperStop(h.oid); err != nil {
			return fmt.Errorf("cycle %d stop: %w", i, err)
		}
		h.waitForRest(time.Second)
		stops = append(stops, time.Since(t)-300*time.Millisecond)
		h.status(fmt.Sprintf("cycle %d", i))
	}
	report("click start to first step", starts)
	report("click stop to motion ended", stops)
	fmt.Printf("cycles=%d no_motion=%d\n", cycles, failed)
	return nil
}

func (h *harness) routine() error {
	segments := []mcu.StepperRoutineSegment{
		{Forward: true, SpeedHz: h.speed, Steps: *segSteps},
		{Forward: false, SpeedHz: h.speed, Steps: *segSteps},
	}
	from := h.position()
	t := time.Now()
	if err := h.s.StartStepperRoutine(h.oid, segments); err != nil {
		return err
	}
	wait, moved := h.waitForMotion(from, 3*time.Second)
	fmt.Printf("routine first_step=%s moved=%v\n", time.Since(t).Round(time.Millisecond), moved)
	_ = wait

	deadline := time.Now().Add(10 * time.Second)
	lastChange := time.Now()
	last := h.position()
	longestStall := time.Duration(0)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		p := h.position()
		if p != last {
			last, lastChange = p, time.Now()
		} else if d := time.Since(lastChange); d > longestStall {
			longestStall = d
		}
	}
	fmt.Printf("routine longest_stall=%s states=%+v\n", longestStall.Round(time.Millisecond), h.s.RoutineStates())

	t = time.Now()
	if err := h.s.StepperStop(h.oid); err != nil {
		return err
	}
	callDur := time.Since(t)
	rest := h.waitForRest(2 * time.Second)
	fmt.Printf("routine stop call=%s last_step_after=%s\n", callDur.Round(time.Millisecond), rest.Round(time.Millisecond))
	return nil
}

func parseOIDs() []int {
	var oids []int
	for _, part := range strings.Split(*oidList, ",") {
		if oid, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			oids = append(oids, oid)
		}
	}
	return oids
}

// parallel runs fn for each oid at once, as the UI's Promise.all group actions do.
func parallel(oids []int, fn func(oid int) error) ([]time.Duration, error) {
	durs := make([]time.Duration, len(oids))
	errs := make([]error, len(oids))
	var wg sync.WaitGroup
	for i, oid := range oids {
		wg.Add(1)
		go func(i, oid int) {
			defer wg.Done()
			t := time.Now()
			errs[i] = fn(oid)
			durs[i] = time.Since(t)
		}(i, oid)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("oid %d: %w", oids[i], err)
		}
	}
	return durs, nil
}

func (h *harness) positions(oids []int) map[int]int64 {
	out := make(map[int]int64, len(oids))
	for _, oid := range oids {
		pos, err := h.s.StepperPosition(oid)
		if err != nil {
			fmt.Printf("  position oid %d: %v\n", oid, err)
		}
		out[oid] = pos
	}
	return out
}

func (h *harness) group(cycles int) error {
	oids := parseOIDs()
	var starts, stops []time.Duration
	failed := 0
	for i := 0; i < cycles; i++ {
		from := h.positions(oids)
		durs, err := parallel(oids, func(oid int) error { return h.s.StepperJog(oid, i%2 == 0, h.speed, 0) })
		if err != nil {
			return fmt.Errorf("cycle %d start: %w", i, err)
		}
		starts = append(starts, durs...)
		time.Sleep(600 * time.Millisecond)
		now := h.positions(oids)
		for _, oid := range oids {
			if now[oid] == from[oid] {
				failed++
				fmt.Printf("cycle %d oid %d: NO MOTION\n", i, oid)
			}
		}
		durs, err = parallel(oids, h.s.StepperStop)
		if err != nil {
			return fmt.Errorf("cycle %d stop: %w", i, err)
		}
		stops = append(stops, durs...)
		time.Sleep(150 * time.Millisecond)
		h.status(fmt.Sprintf("cycle %d", i))
	}
	report("group jog call", starts)
	report("group stop call", stops)
	fmt.Printf("cycles=%d motors=%d no_motion=%d\n", cycles, len(oids), failed)
	return nil
}

// routineHold runs a routine on each motor so someone can watch, until told to stop.
func (h *harness) routineHold() error {
	oids := parseOIDs()
	segments := []mcu.StepperRoutineSegment{
		{Forward: true, SpeedHz: h.speed, Steps: *segSteps},
		{Forward: false, SpeedHz: h.speed, Steps: *segSteps},
	}
	if _, err := parallel(oids, func(oid int) error { return h.s.StartStepperRoutine(oid, segments) }); err != nil {
		return err
	}
	defer func() {
		_, _ = parallel(oids, h.s.StepperStop)
		fmt.Println("stopped")
	}()
	start := time.Now()
	for time.Since(start) < *holdLimit {
		if *stopFile != "" {
			if _, err := os.Stat(*stopFile); err == nil {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
		fmt.Printf("routines t=%s positions=%v states=%+v\n", time.Since(start).Round(time.Second), h.positions(oids), h.s.RoutineStates())
	}
	return nil
}

func (h *harness) routineGroup() error {
	oids := parseOIDs()
	segments := []mcu.StepperRoutineSegment{
		{Forward: true, SpeedHz: h.speed, Steps: *segSteps},
		{Forward: false, SpeedHz: h.speed, Steps: *segSteps},
	}
	if _, err := parallel(oids, func(oid int) error { return h.s.StartStepperRoutine(oid, segments) }); err != nil {
		return err
	}
	stalls := make(map[int]time.Duration)
	lastChange := make(map[int]time.Time)
	last := h.positions(oids)
	for _, oid := range oids {
		lastChange[oid] = time.Now()
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		now := h.positions(oids)
		for _, oid := range oids {
			if now[oid] != last[oid] {
				last[oid], lastChange[oid] = now[oid], time.Now()
			} else if d := time.Since(lastChange[oid]); d > stalls[oid] {
				stalls[oid] = d
			}
		}
	}
	fmt.Printf("routine-group longest_stalls=%v states=%+v\n", stalls, h.s.RoutineStates())
	h.status("before stop")
	durs, err := parallel(oids, h.s.StepperStop)
	if err != nil {
		return err
	}
	report("routine-group stop call", durs)
	return nil
}
