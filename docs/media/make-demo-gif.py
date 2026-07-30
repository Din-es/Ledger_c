"""Render the ledger demo as a terminal-style animated GIF.

The text is real output captured from the CLI; only the presentation is
synthetic.
"""
import sys
from PIL import Image, ImageDraw, ImageFont

OUT = sys.argv[1]

W, H = 820, 470
BG = (26, 26, 25)
CHROME = (36, 36, 34)
PAD_X, PAD_Y = 26, 62
LINE_H = 26
FPS_MS = 90

C_PROMPT = (93, 202, 165)
C_CMD = (232, 232, 230)
C_DIM = (122, 122, 114)
C_GREEN = (27, 175, 122)
C_RED = (227, 73, 72)
C_AMBER = (237, 161, 39)
C_PATH = (154, 154, 146)

mono = ImageFont.truetype(r"C:\Windows\Fonts\consola.ttf", 16)
monob = ImageFont.truetype(r"C:\Windows\Fonts\consolab.ttf", 16)

# (kind, text, colour) — kind: cmd (typed), out (instant), gap
SCRIPT = [
    ("cmd", "ledger bind src/checkout.go:4-7 --note idempotency-key", C_CMD),
    ("out", "bound idempotency-key -> src/checkout.go:4-7 @ 65c40c6", C_GREEN),
    ("out", "   rationale: docs/decisions/idempotency-key.md", C_DIM),
    ("gap", "", None),
    ("out", "# a month later: file moved to internal/billing/, package renamed", C_DIM),
    ("cmd", "ledger resolve idempotency-key", C_CMD),
    ("out", "[fresh]  internal/billing/checkout.go:6-9   conf 1.00", C_GREEN),
    ("out", "   followed rename: src/checkout.go -> internal/billing/", C_PATH),
    ("gap", "", None),
    ("out", "# a new dev 'improves' it to a random UUID", C_DIM),
    ("cmd", "ledger verify --since main", C_CMD),
    ("out", "[STALE] idempotency-key", C_RED),
    ("out", "        code changed, but the decision note was not", C_AMBER),
    ("out", "verify failed: 1 decision(s) changed without revisiting", C_RED),
    ("gap", "", None),
    ("out", "  the pull request is blocked.", C_CMD),
]


def base_frame():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.rectangle([0, 0, W, 40], fill=CHROME)
    for i, col in enumerate([(227, 93, 88), (237, 185, 79), (99, 191, 122)]):
        d.ellipse([20 + i * 22, 15, 32 + i * 22, 27], fill=col)
    d.text((W // 2 - 92, 12), "ledger — decision gate", font=mono, fill=(140, 140, 132))
    return img, d


def draw_lines(d, lines, cursor=None):
    y = PAD_Y
    for kind, text, col in lines:
        if kind == "gap":
            y += LINE_H // 2
            continue
        x = PAD_X
        if kind == "cmd":
            d.text((x, y), "$", font=monob, fill=C_PROMPT)
            x += 18
        d.text((x, y), text, font=mono, fill=col)
        if cursor is not None and (kind, text) == cursor:
            w = d.textlength(text, font=mono)
            d.rectangle([x + w + 2, y + 2, x + w + 10, y + 18], fill=C_CMD)
        y += LINE_H


frames = []
shown = []
for kind, text, col in SCRIPT:
    if kind == "cmd":
        # type it out a couple of characters at a time
        for n in range(0, len(text) + 1, 2):
            img, d = base_frame()
            draw_lines(d, shown + [(kind, text[:n], col)], cursor=(kind, text[:n]))
            frames.append(img)
        shown.append((kind, text, col))
        for _ in range(4):
            img, d = base_frame()
            draw_lines(d, shown)
            frames.append(img)
    else:
        shown.append((kind, text, col))
        img, d = base_frame()
        draw_lines(d, shown)
        hold = 8 if text.startswith(("[STALE]", "verify failed", "  the pull")) else 4
        for _ in range(hold):
            frames.append(img)

# hold the final state so the punchline lands before looping
img, d = base_frame()
draw_lines(d, shown)
for _ in range(28):
    frames.append(img)

frames = [f.convert("P", palette=Image.ADAPTIVE, colors=32) for f in frames]
frames[0].save(
    OUT, save_all=True, append_images=frames[1:],
    duration=FPS_MS, loop=0, optimize=True, disposal=2,
)
print(f"{OUT}  frames={len(frames)}")
