import { Stage } from './kit/stage'
import { Prompt, Window } from './kit/window'
import './cover.css'

// README cover for gitsy, a still; regenerate with `npm run export`.
// The hero is a terminal running the gitsy TUI after a run: three scan
// directories, every repo fetched, three fast-forwarded, one fetch failure
// selected. The terminal is under 104 columns, so (as in the real TUI) the
// context rail is hidden. A small terminal straddles its right edge over the
// ledger's empty status margin and shows the same run headless: `gitsy sync`
// and its one-line summary. Every string matches the real TUI and plain output.

type Tone = 'hi' | 'med' | 'lo' | 'brand' | 'ok' | 'warn' | 'info'
type Seg = [string, Tone?, boolean?]

const REPO_W = 14
const NUM_W = 4

type Repo = { name: string; status: Seg[]; selected?: boolean }
type Group = { path: string; count: number; repos: Repo[] }

const CLEAN: Seg[] = [['main ✓ clean', 'lo']]
const synced = (n: number): Seg[] => [['main ✓ clean', 'lo'], [' · ', 'lo'], [`synced ${n} commit${n === 1 ? '' : 's'}`, 'ok']]

const GROUPS: Group[] = [
  {
    path: '~/work/oss',
    count: 4,
    repos: [
      { name: 'api', status: synced(3) },
      { name: 'cli', status: synced(1) },
      { name: 'docs', status: [['main · 2 modified · 1 untracked', 'info']] },
      { name: 'web · web-auth', status: [['feat/auth ↑2', 'info']] },
    ],
  },
  {
    path: '~/work/acme',
    count: 4,
    repos: [
      { name: 'billing', selected: true, status: [['main ✓ clean', 'ok', true], [' · ', 'lo', true], ['⚠ remote auth needed', 'warn', true]] },
      { name: 'infra', status: CLEAN },
      { name: 'payments', status: [['main · 1 added', 'info']] },
      { name: 'search', status: synced(4) },
    ],
  },
  {
    path: '~/src',
    count: 6,
    repos: [
      { name: 'dotfiles', status: [['main · 1 modified', 'info']] },
      { name: 'notes', status: CLEAN },
      { name: 'scripts', status: CLEAN },
      { name: 'site', status: CLEAN },
      { name: 'talks', status: CLEAN },
      { name: 'tools', status: CLEAN },
    ],
  },
]

function Line({ segs, className, ...rest }: { segs: Seg[]; className?: string; [k: `data-${string}`]: unknown }) {
  return (
    <div className={`gc-line${className ? ` ${className}` : ''}`} {...rest}>
      {segs.map(([text, tone, bold], i) => (
        <span key={i} className={tone ? `t-${tone}` : undefined} data-bold={bold || undefined}>
          {text}
        </span>
      ))}
    </div>
  )
}

function Ledger() {
  let n = 0
  return (
    <>
      {GROUPS.map((group) => (
        <div key={group.path}>
          <Line segs={[[group.path, 'med', true], ['  '], [`${group.count} repos`, 'lo']]} />
          {group.repos.map((repo) => {
            n += 1
            const num = String(n).padStart(NUM_W - 1)
            const lead: Seg[] = repo.selected
              ? [['›', 'brand', true], [num, 'hi', true], ['  '], [repo.name.padEnd(REPO_W), 'hi', true], ['  ']]
              : [[' '], [num, 'lo'], ['  '], [repo.name.padEnd(REPO_W), 'hi'], ['  ']]
            return <Line key={repo.name} segs={[...lead, ...repo.status]} className={repo.selected ? 'gc-sel' : undefined} data-target={repo.selected ? 'selected' : undefined} />
          })}
        </div>
      ))}
    </>
  )
}

export default function GitsyCover() {
  return (
    <Stage width={972} height={500} theme="dark" className="gc">
      {() => (
        <>
          <Window chrome="terminal" lights="grey" className="gc-hero" body={{ className: 'gc-body' }}>
            <div className="gc-head">
              <Line segs={[['gitsy · 3 directories', 'brand', true]]} />
              <Line segs={[['14/14 done', 'lo']]} />
            </div>
            <Line segs={[['14 repos', 'lo'], [' · ', 'lo'], ['1 ahead', 'info'], [' · ', 'lo'], ['1 fetch failed', 'warn'], [' · ', 'lo'], ['3 changed', 'info'], [' · ', 'lo'], ['3 synced', 'ok']]} />
            <Line segs={[['[Repositories]', 'brand', true], ['  Worktrees  tab', 'lo']]} />
            <Line className="gc-colhead" segs={[['#'.padEnd(NUM_W + 2) + 'Repository'.padEnd(REPO_W + 2) + 'Branch / status', 'lo', true]]} />
            <div className="gc-rule" />
            <Ledger />
          </Window>

          <Window chrome="terminal" lights="grey" className="gc-sat" body={{ className: 'gc-sat-body' }}>
            <Prompt path="~">gitsy sync</Prompt>
            <div className="gc-out">Synced 3 of 14 repos. 10 up to date, 1 failed.</div>
          </Window>
        </>
      )}
    </Stage>
  )
}
