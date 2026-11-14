// Draws the app icon the Gangway shows for the viewer (its tile on login.n8p.xyz, the
// sign-in header, emails): the three-brick mark on a dark rounded square. Writes logo.svg
// and logo.png next to this file; `mage keeperTheme` sends the PNG.
//
//   node brand/make-logo.mjs
//
// No dependencies: the shapes are only rounded rectangles, so the PNG is drawn here.
import { writeFileSync } from "node:fs";
import { crc32, deflateSync } from "node:zlib";

const GRID = 64; // the mark's own grid
const SIZE = 256; // PNG pixels
const SAMPLES = 4; // per pixel, each way
const GROUND = "#1c1917";
const BRICKS = ["#3987e5", "#ea580c", "#199e70"]; // the dark theme's series-1, accent, series-3
const SCALE = 0.76; // the mark inside the square
// A little squarer than the portal tile (14 of 56), so the tile's colour never shows round it.
const CORNER = 14;

const shapes = [{ x: 0, y: 0, w: GRID, h: GRID, r: CORNER, fill: GROUND }];
BRICKS.forEach((fill, i) => {
  const body = [6 + i * 18, 23, 16, 25, 2.5];
  const stud = [9.5 + i * 18, 16, 9, 9, 1.5];
  for (const [x, y, w, h, r] of [body, stud]) {
    const at = (v) => GRID / 2 + (v - GRID / 2) * SCALE;
    shapes.push({
      x: at(x),
      y: at(y),
      w: w * SCALE,
      h: h * SCALE,
      r: r * SCALE,
      fill,
    });
  }
});

const n = (v) => String(Math.round(v * 100) / 100);
const svg = [
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${GRID} ${GRID}">`,
  ...shapes.map(
    (s) =>
      `  <rect x="${n(s.x)}" y="${n(s.y)}" width="${n(s.w)}" height="${n(s.h)}" rx="${n(s.r)}" fill="${s.fill}"/>`,
  ),
  "</svg>",
  "",
].join("\n");
writeFileSync(new URL("./logo.svg", import.meta.url), svg);

const inside = (s, px, py) => {
  if (px < s.x || px > s.x + s.w || py < s.y || py > s.y + s.h) return false;
  const dx = Math.max(s.x + s.r - px, px - (s.x + s.w - s.r), 0);
  const dy = Math.max(s.y + s.r - py, py - (s.y + s.h - s.r), 0);
  return dx * dx + dy * dy <= s.r * s.r;
};
const rgb = (hex) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));

// One filter byte per row, then RGBA.
const raw = Buffer.alloc(SIZE * (SIZE * 4 + 1));
for (let y = 0; y < SIZE; y++) {
  for (let x = 0; x < SIZE; x++) {
    const sum = [0, 0, 0];
    let covered = 0;
    for (let sy = 0; sy < SAMPLES; sy++) {
      for (let sx = 0; sx < SAMPLES; sx++) {
        const px = ((x + (sx + 0.5) / SAMPLES) * GRID) / SIZE;
        const py = ((y + (sy + 0.5) / SAMPLES) * GRID) / SIZE;
        const top = shapes.findLast((s) => inside(s, px, py));
        if (!top) continue;
        covered++;
        rgb(top.fill).forEach((c, i) => (sum[i] += c));
      }
    }
    const at = y * (SIZE * 4 + 1) + 1 + x * 4;
    if (covered) sum.forEach((c, i) => (raw[at + i] = Math.round(c / covered)));
    raw[at + 3] = Math.round((covered * 255) / (SAMPLES * SAMPLES));
  }
}

const chunk = (type, data) => {
  const body = Buffer.concat([Buffer.from(type, "latin1"), data]);
  const out = Buffer.alloc(body.length + 8);
  out.writeUInt32BE(data.length, 0);
  body.copy(out, 4);
  out.writeUInt32BE(crc32(body), body.length + 4);
  return out;
};
const header = Buffer.alloc(13);
header.writeUInt32BE(SIZE, 0);
header.writeUInt32BE(SIZE, 4);
header.set([8, 6, 0, 0, 0], 8); // 8 bits, RGBA
const png = Buffer.concat([
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  chunk("IHDR", header),
  chunk("IDAT", deflateSync(raw, { level: 9 })),
  chunk("IEND", Buffer.alloc(0)),
]);
writeFileSync(new URL("./logo.png", import.meta.url), png);
console.log(`logo.svg and logo.png (${SIZE}×${SIZE}, ${png.length} bytes)`);
