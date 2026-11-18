# README cover

Source for `assets/gitsy-cover.webp`: a still of the gitsy TUI with a `gitsy sync` satellite, built with a subset of the Stageshow kit (`kit/`, MIT, see `kit/LICENSE`). Colors are gitsy's dark tokens from `DESIGN.md`. Font: JetBrains Mono (`fonts/OFL.txt`).

Needs Node 20+, Google Chrome (or `CHROME_PATH`), and `ffmpeg` with libwebp.

```bash
cd docs/cover
npm install
npm run dev      # preview at the printed URL
npm run export   # writes ../../assets/gitsy-cover.webp
```

Keep the strings in `GitsyCover.tsx` in sync with the TUI and `gitsy sync` output when they change.
