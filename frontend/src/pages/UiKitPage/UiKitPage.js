import { CircleStop, Lightbulb, Search, Settings2 } from 'lucide-react';

import { SWATCHES, THEMES, useUiKitPageLogic } from './UiKitPage.logic';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from '@/components/ui/command';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Progress } from '@/components/ui/progress';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Slider } from '@/components/ui/slider';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Toaster } from '@/components/ui/sonner';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

/**
 * The design system's parts in one place, in both themes: the reference for building the
 * new pages (docs/ui-ux-spec.md). Temporary in the nav until the new app frame.
 */
const UiKitPage = () => {
  const logic = useUiKitPageLogic();
  return (
    <TooltipProvider>
      <div className="ds rounded-2xl bg-background p-7 text-foreground">
        <div className="flex flex-col gap-6">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div className="flex flex-col gap-1">
              <h1 className="font-display text-[28px] font-bold tracking-tight">UI kit</h1>
              <p className="text-sm text-muted-foreground">
                The design system for the new UI: shadcn/ui on our tokens. Showing the{' '}
                {logic.resolved} theme.
              </p>
            </div>
            <Tabs value={logic.theme} onValueChange={logic.setTheme}>
              <TabsList>
                {THEMES.map((t) => (
                  <TabsTrigger key={t.id} value={t.id}>
                    {t.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Type</CardTitle>
                <CardDescription>
                  Space Grotesk titles, Inter text, JetBrains Mono for live numbers.
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-2">
                <span className="font-display text-[34px] font-bold tracking-tight">
                  Ready to sort
                </span>
                <span className="font-display text-[28px] font-bold">Run</span>
                <span className="font-display text-lg font-semibold">Recently sorted</span>
                <span className="text-[15px]">Brick 2 × 4, red · layer 1, bin 4</span>
                <span className="text-[13px] text-muted-foreground">
                  Secondary text, 13 px, passes 4.5:1
                </span>
                <span className="font-mono text-lg">12.1 V · 9.9 fps · 14.0 rpm</span>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Color tokens</CardTitle>
                <CardDescription>
                  Switch the theme above: every token changes with it.
                </CardDescription>
              </CardHeader>
              <CardContent className="grid grid-cols-3 gap-3 sm:grid-cols-4">
                {SWATCHES.map((s) => (
                  <div key={s.name} className="flex flex-col gap-1.5">
                    <span className={`h-12 rounded-md border ${s.className}`} />
                    <span className="text-xs text-muted-foreground">{s.name}</span>
                  </div>
                ))}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Buttons and status</CardTitle>
                <CardDescription>40 px on a mouse, 44 px on touch screens.</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                <div className="flex flex-wrap gap-2">
                  <Button>Start sorting</Button>
                  <Button variant="outline">Home</Button>
                  <Button variant="secondary">Pause</Button>
                  <Button variant="ghost">Cancel</Button>
                  <Button variant="destructive">
                    <CircleStop /> Stop
                  </Button>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="outline" size="icon" aria-label="Settings">
                        <Settings2 />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Icon buttons always get a label</TooltipContent>
                  </Tooltip>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Badge variant="running">● Sorting · run 14</Badge>
                  <Badge variant="idle">● Ready</Badge>
                  <Badge variant="warning">! Needs attention</Badge>
                  <Badge variant="destructive">Error</Badge>
                  <Badge>Accent</Badge>
                  <Badge variant="outline">COM4</Badge>
                </div>
                <div className="flex flex-col gap-1.5">
                  <span className="text-[13px] text-muted-foreground">
                    Capturing C3 across a section · 12 of 36
                  </span>
                  <Progress value={33} />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Fields</CardTitle>
                <CardDescription>What setting fields will be built from.</CardDescription>
                <CardAction>
                  <Button variant="link" className="px-0">
                    Reset to 2°
                  </Button>
                </CardAction>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="kit-exit">Exit pulse (°)</Label>
                  <Input
                    id="kit-exit"
                    type="number"
                    value={logic.exitPulse}
                    onChange={(e) => logic.setExitPulse(Number(e.target.value))}
                    className="max-w-32 font-mono"
                  />
                  <Slider
                    value={[logic.exitPulse]}
                    min={0.1}
                    max={10}
                    step={0.1}
                    onValueChange={([v]) => logic.setExitPulse(v)}
                  />
                </div>
                <div className="flex flex-wrap items-center gap-6">
                  <div className="flex items-center gap-2">
                    <Switch
                      id="kit-light"
                      checked={logic.lightOn}
                      onCheckedChange={logic.setLightOn}
                    />
                    <Label htmlFor="kit-light">
                      <Lightbulb className="size-4" /> Light
                    </Label>
                  </div>
                  <div className="flex items-center gap-2">
                    <Checkbox id="kit-sorting" defaultChecked />
                    <Label htmlFor="kit-sorting">Turn on when sorting starts</Label>
                  </div>
                </div>
                <div className="flex flex-col gap-2">
                  <Label>Camera size</Label>
                  <Select defaultValue="1280x720">
                    <SelectTrigger className="w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1280x720">1280 × 720</SelectItem>
                      <SelectItem value="960x540">960 × 540</SelectItem>
                      <SelectItem value="640x480">640 × 480</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Tabs and table</CardTitle>
                <CardDescription>Part tabs, as on Machine › Channels.</CardDescription>
              </CardHeader>
              <CardContent>
                <Tabs defaultValue="c3">
                  <TabsList>
                    {['c1', 'c2', 'c3', 'c4'].map((c) => (
                      <TabsTrigger key={c} value={c}>
                        {c.toUpperCase()}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                  <TabsContent value="c3" className="pt-3">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Setting</TableHead>
                          <TableHead className="text-right">Value</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {[
                          ['Speed', '14.0 rpm'],
                          ['Gearing', '10.833'],
                          ['Sections', '10'],
                        ].map(([k, v]) => (
                          <TableRow key={k}>
                            <TableCell>{k}</TableCell>
                            <TableCell className="text-right font-mono">{v}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Overlays</CardTitle>
                <CardDescription>Dialog, popover, toast and the search palette.</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-2">
                <Button variant="outline" onClick={() => logic.setDialogOpen(true)}>
                  Quick-edit a setting
                </Button>
                <Popover>
                  <PopoverTrigger asChild>
                    <Button variant="outline">
                      <Badge variant="running">● Sorting</Badge>
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="flex flex-col gap-2">
                    <span className="font-semibold">Sorting · run 14</span>
                    <Separator />
                    <span className="text-sm text-muted-foreground">
                      Board COM4 · motor power 12.1 V · light on
                    </span>
                  </PopoverContent>
                </Popover>
                <Button variant="outline" onClick={logic.toastSaved}>
                  Toast
                </Button>
                <Button variant="outline" onClick={logic.toastWarn}>
                  Warning toast
                </Button>
                <Button variant="outline" onClick={() => logic.setSearchOpen(true)}>
                  <Search /> Search
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>

        <Dialog open={logic.dialogOpen} onOpenChange={logic.setDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Exit pulse</DialogTitle>
              <DialogDescription>
                How far a piece at a channel&apos;s exit is nudged each pulse. Smaller is gentler.
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                value={logic.exitPulse}
                onChange={(e) => logic.setExitPulse(Number(e.target.value))}
                className="w-28 font-mono"
              />
              <span className="text-muted-foreground">°</span>
              <Badge variant="running" className="ml-auto">
                Applies while sorting
              </Badge>
            </div>
            <DialogFooter>
              <DialogClose asChild>
                <Button variant="outline">Cancel</Button>
              </DialogClose>
              <DialogClose asChild>
                <Button onClick={logic.toastSaved}>Save</Button>
              </DialogClose>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <CommandDialog open={logic.searchOpen} onOpenChange={logic.setSearchOpen}>
          <CommandInput placeholder="Go to a page, change a setting, run an action…" />
          <CommandList>
            <CommandEmpty>Nothing matches.</CommandEmpty>
            <CommandGroup heading="Settings">
              <CommandItem>
                Exit pulse <CommandShortcut>2°</CommandShortcut>
              </CommandItem>
              <CommandItem>
                Drop-zone pulse <CommandShortcut>30°</CommandShortcut>
              </CommandItem>
            </CommandGroup>
            <CommandGroup heading="Pages">
              <CommandItem>Machine › Channels</CommandItem>
              <CommandItem>Machine › Cameras › C3</CommandItem>
            </CommandGroup>
            <CommandGroup heading="Actions">
              <CommandItem>Blink the light</CommandItem>
            </CommandGroup>
          </CommandList>
        </CommandDialog>

        <Toaster />
      </div>
    </TooltipProvider>
  );
};

export default UiKitPage;
