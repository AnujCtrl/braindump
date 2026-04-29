// bands.jsx — Band 1 chart, Band 2 contribution graph, Band 3 pile, report modal.

const { useState: useStateB, useMemo: useMemoB, useEffect: useEffectB, useRef: useRefB } = React;

// ── Band 1: Burndown chart (Linear-style slope) ─────────────────────────────
function BurndownChart({ data, palette, accentName }) {
  const w = 480, h = 160, pad = { l: 28, r: 16, t: 14, b: 22 };
  const innerW = w - pad.l - pad.r;
  const innerH = h - pad.t - pad.b;
  const max = Math.max(1, data.total);
  const x = (d) => pad.l + (d / 6) * innerW;
  const y = (v) => pad.t + (1 - v / max) * innerH;
  const todayX = x(data.todayIdx);
  const idealPath = `M ${x(0)} ${y(data.points[0].ideal)} L ${x(6)} ${y(data.points[6].ideal)}`;
  const actualPts = data.points.filter((p) => p.actual);
  const actualPath = actualPts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${x(p.day)} ${y(p.remaining)}`).join(' ');
  const projPts = [actualPts[actualPts.length - 1], { day: 6, remaining: data.points[6].ideal }];
  // Simple projection from current slope:
  const slope = actualPts.length > 1
    ? (actualPts[actualPts.length - 1].remaining - actualPts[0].remaining) / (actualPts[actualPts.length - 1].day - actualPts[0].day)
    : 0;
  const projEnd = Math.max(0, actualPts[actualPts.length - 1].remaining + slope * (6 - data.todayIdx));
  const projPath = `M ${x(data.todayIdx)} ${y(actualPts[actualPts.length - 1].remaining)} L ${x(6)} ${y(projEnd)}`;
  const days = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];

  return (
    <svg viewBox={`0 0 ${w} ${h}`} width="100%" height={h} style={{ display: 'block' }}>
      {/* y gridlines */}
      {[0, 0.5, 1].map((v) => (
        <line key={v} x1={pad.l} y1={pad.t + v * innerH} x2={w - pad.r} y2={pad.t + v * innerH}
          stroke="#15171a" strokeWidth="1" />
      ))}
      {/* day labels + today marker */}
      {days.map((d, i) => (
        <text key={i} x={x(i)} y={h - 6} textAnchor="middle"
          fill={i === data.todayIdx ? '#f0f1f3' : 'rgba(255,255,255,0.32)'}
          fontFamily="JetBrains Mono, ui-monospace, monospace" fontSize="9">{d}</text>
      ))}
      <line x1={todayX} y1={pad.t} x2={todayX} y2={h - pad.b}
        stroke="rgba(255,255,255,0.18)" strokeWidth="1" strokeDasharray="2 3" />
      {/* y labels */}
      <text x={pad.l - 6} y={y(max) + 3} textAnchor="end"
        fill="rgba(255,255,255,0.32)" fontFamily="JetBrains Mono, ui-monospace, monospace" fontSize="9">{max}</text>
      <text x={pad.l - 6} y={y(0) + 3} textAnchor="end"
        fill="rgba(255,255,255,0.32)" fontFamily="JetBrains Mono, ui-monospace, monospace" fontSize="9">0</text>
      {/* ideal line */}
      <path d={idealPath} stroke="rgba(255,255,255,0.18)" strokeWidth="1" strokeDasharray="3 4" fill="none" />
      {/* projection */}
      <path d={projPath} stroke={palette.good} strokeWidth="1" strokeDasharray="2 3" fill="none" opacity="0.55" />
      {/* actual */}
      <path d={actualPath} stroke={palette.good} strokeWidth="1.6" fill="none" />
      {actualPts.map((p, i) => (
        <circle key={i} cx={x(p.day)} cy={y(p.remaining)} r="2.4" fill={palette.good} />
      ))}
      {/* end-of-week label */}
      <text x={x(6)} y={y(projEnd) - 6} textAnchor="end"
        fill={palette.good} fontFamily="JetBrains Mono, ui-monospace, monospace" fontSize="9">
        ~{Math.round(projEnd)} left
      </text>
    </svg>
  );
}

// ── Band 2: Contribution graph (capture vs completion) ──────────────────────
function ContribGraph({ history, kind, palette, days = 28 }) {
  // Render 4 weeks × 7 days. Cells colored by saturation; mono labels.
  const cells = history.slice(-days);
  const max = Math.max(1, ...cells.map((c) => c[kind === 'capture' ? 'capture' : 'complete']));
  const cellSize = 14, gap = 3;
  const cols = Math.ceil(cells.length / 7);
  const w = cols * (cellSize + gap);
  const h = 7 * (cellSize + gap) + 16;
  const accent = kind === 'capture' ? palette.good : palette.flow;

  const intensity = (v) => {
    if (v === 0) return 0;
    const n = v / max;
    if (n < 0.25) return 0.25;
    if (n < 0.5) return 0.45;
    if (n < 0.75) return 0.7;
    return 1;
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ display: 'block' }}>
        {cells.map((c, i) => {
          const col = Math.floor(i / 7);
          const row = c.dow; // 0 sun..6 sat
          const x = col * (cellSize + gap);
          const y = row * (cellSize + gap);
          const v = c[kind === 'capture' ? 'capture' : 'complete'];
          const a = intensity(v);
          const fill = a === 0 ? '#0e1012' : accent;
          return (
            <rect key={i} x={x} y={y} width={cellSize} height={cellSize} rx="2"
              fill={fill} fillOpacity={a || 1}
              stroke="#0a0a0b" strokeWidth="1">
              <title>{`${c.label}: ${v} ${kind === 'capture' ? 'captured' : 'completed'}`}</title>
            </rect>
          );
        })}
        {/* day-of-week labels */}
        {['M', 'W', 'F'].map((d, i) => (
          <text key={d} x={-4} y={(i * 2 + 1) * (cellSize + gap) + cellSize - 3}
            textAnchor="end" fill="rgba(255,255,255,0.32)"
            fontFamily="JetBrains Mono, ui-monospace, monospace" fontSize="8">{d}</text>
        ))}
      </svg>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 9, color: 'rgba(255,255,255,0.4)' }}>
        <span>less</span>
        {[0.18, 0.4, 0.6, 0.85, 1].map((a, i) => (
          <span key={i} style={{ width: 10, height: 10, borderRadius: 2, background: i === 0 ? '#0e1012' : accent, opacity: i === 0 ? 1 : a, border: '1px solid #0a0a0b' }} />
        ))}
        <span>more</span>
      </div>
    </div>
  );
}

// ── Band 2: 7/14/28-day side-by-side capture vs completion bars ─────────────
function VolumeBars({ history, palette }) {
  const windows = [7, 14, 28];
  const sums = windows.map((w) => {
    const slice = history.slice(-w);
    return {
      window: w,
      capture: slice.reduce((a, b) => a + b.capture, 0),
      complete: slice.reduce((a, b) => a + b.complete, 0),
    };
  });
  const max = Math.max(1, ...sums.flatMap((s) => [s.capture, s.complete]));

  return (
    <div style={{ display: 'flex', gap: 18 }}>
      {sums.map((s) => (
        <div key={s.window} style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div style={{ fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 9, letterSpacing: '0.05em', color: 'rgba(255,255,255,0.45)', textTransform: 'uppercase' }}>
            ROLLING {s.window}D
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 10, height: 64 }}>
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
              <div style={{ height: `${(s.capture / max) * 56}px`, background: palette.good, borderRadius: 1, transition: 'height .4s ease' }} />
              <div style={{ fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 16, color: '#f0f1f3' }}>{s.capture}</div>
              <div style={{ fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 9, color: palette.good, opacity: 0.7 }}>captured</div>
            </div>
            <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 4 }}>
              <div style={{ height: `${(s.complete / max) * 56}px`, background: palette.flow, opacity: 0.6, borderRadius: 1, transition: 'height .4s ease' }} />
              <div style={{ fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 16, color: '#f0f1f3' }}>{s.complete}</div>
              <div style={{ fontFamily: 'JetBrains Mono, ui-monospace, monospace', fontSize: 9, color: palette.flow, opacity: 0.7 }}>completed</div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Band 3: The Pile ────────────────────────────────────────────────────────
const STATUS_LABELS = {
  'inbox': 'inbox',
  'this-week': 'this week',
  'done': 'done',
  'rollover': 'parked',
  'stale': 'rest pile',
};

function PileRow({ todo, palette, onAction }) {
  const [open, setOpen] = useStateB(false);
  const ageColor = todo.age > 14 ? palette.warn : 'rgba(255,255,255,0.45)';
  const statusColor = {
    'done': 'rgba(255,255,255,0.35)',
    'this-week': palette.good,
    'rollover': palette.warn,
    'stale': palette.warn,
    'inbox': 'rgba(255,255,255,0.55)',
  }[todo.status];
  return (
    <div className="pile-row" data-done={todo.status === 'done' ? '1' : '0'}>
      <div className="pile-cell pile-status">
        <span className="pile-dot" style={{ background: statusColor, opacity: todo.status === 'done' ? 0.4 : 1 }} />
        <span className="pile-status-label">{STATUS_LABELS[todo.status]}</span>
      </div>
      <div className="pile-cell pile-text" style={{ opacity: todo.status === 'done' ? 0.45 : 1, textDecoration: todo.status === 'done' ? 'line-through' : 'none' }}>
        {todo.t}
      </div>
      <div className="pile-cell pile-tag">
        <span className="pile-tag-chip">{todo.tag}</span>
      </div>
      <div className="pile-cell pile-src">{todo.src}</div>
      <div className="pile-cell pile-age" style={{ color: ageColor }}>{todo.age}d</div>
      <div className="pile-cell pile-actions">
        <button className="pile-action" onClick={() => onAction('done', todo)} title="Mark done"><DoneIcon /></button>
        <button className="pile-action" onClick={() => onAction('this-week', todo)} title="To this week"><ArrowIcon /></button>
        <button className="pile-action" onClick={() => onAction('next-week', todo)} title="Park for next week"><ParkIcon /></button>
        <button className="pile-action" onClick={() => onAction('edit', todo)} title="Edit"><EditIcon /></button>
      </div>
    </div>
  );
}

function DoneIcon() { return <svg width="11" height="11" viewBox="0 0 11 11"><path d="M2 6 L4.5 8.5 L9 3" stroke="currentColor" strokeWidth="1.4" fill="none" strokeLinecap="round" strokeLinejoin="round" /></svg>; }
function ArrowIcon() { return <svg width="11" height="11" viewBox="0 0 11 11"><path d="M2 5.5 H8 M5.5 3 L8 5.5 L5.5 8" stroke="currentColor" strokeWidth="1.4" fill="none" strokeLinecap="round" strokeLinejoin="round" /></svg>; }
function ParkIcon() { return <svg width="11" height="11" viewBox="0 0 11 11"><rect x="2" y="3" width="7" height="6" stroke="currentColor" strokeWidth="1.4" fill="none" rx="1" /><path d="M2 6 H9" stroke="currentColor" strokeWidth="1" /></svg>; }
function EditIcon() { return <svg width="11" height="11" viewBox="0 0 11 11"><path d="M2 9 L2 7.5 L7 2.5 L8.5 4 L3.5 9 Z" stroke="currentColor" strokeWidth="1.2" fill="none" strokeLinejoin="round" /></svg>; }

function Pile({ todos, palette, onAction }) {
  const [filter, setFilter] = useStateB('all');
  const [sort, setSort] = useStateB('age');
  const filters = [
    { v: 'all', label: 'all', count: todos.length },
    { v: 'inbox', label: 'inbox', count: todos.filter(t => t.status === 'inbox').length },
    { v: 'this-week', label: 'this week', count: todos.filter(t => t.status === 'this-week').length },
    { v: 'rollover', label: 'parked', count: todos.filter(t => t.status === 'rollover').length },
    { v: 'stale', label: 'rest pile', count: todos.filter(t => t.status === 'stale').length },
    { v: 'done', label: 'done', count: todos.filter(t => t.status === 'done').length },
  ];
  let visible = filter === 'all' ? todos : todos.filter(t => t.status === filter);
  if (sort === 'age') visible = [...visible].sort((a, b) => b.age - a.age);
  else if (sort === 'tag') visible = [...visible].sort((a, b) => a.tag.localeCompare(b.tag));
  else if (sort === 'status') {
    const order = ['this-week', 'rollover', 'inbox', 'stale', 'done'];
    visible = [...visible].sort((a, b) => order.indexOf(a.status) - order.indexOf(b.status));
  }

  return (
    <div className="pile">
      <div className="pile-toolbar">
        <div className="pile-filters">
          {filters.map((f) => (
            <button key={f.v} className={`pile-filter ${filter === f.v ? 'on' : ''}`} onClick={() => setFilter(f.v)}>
              {f.label} <span className="pile-filter-count">{f.count}</span>
            </button>
          ))}
        </div>
        <div className="pile-sort">
          <span className="pile-sort-label">sort</span>
          {[['age','age'],['tag','tag'],['status','status']].map(([v,l]) => (
            <button key={v} className={`pile-sort-btn ${sort === v ? 'on' : ''}`} onClick={() => setSort(v)}>{l}</button>
          ))}
        </div>
      </div>
      <div className="pile-header">
        <div className="pile-cell pile-status">status</div>
        <div className="pile-cell pile-text">item</div>
        <div className="pile-cell pile-tag">tag</div>
        <div className="pile-cell pile-src">source</div>
        <div className="pile-cell pile-age">age</div>
        <div className="pile-cell pile-actions">actions</div>
      </div>
      <div className="pile-body">
        {visible.length === 0 && (
          <div className="pile-empty">nothing here. quiet doesn't mean broken.</div>
        )}
        {visible.map((t) => (
          <PileRow key={t.id} todo={t} palette={palette} onAction={onAction} />
        ))}
      </div>
    </div>
  );
}

// ── Bi-weekly report modal (receipt aesthetic) ──────────────────────────────
function ReportModal({ metrics, palette, onClose }) {
  return (
    <div className="report-overlay" onClick={onClose}>
      <div className="report-paper" onClick={(e) => e.stopPropagation()}>
        <div className="report-head">
          <div className="report-brand">BRAINDUMP &middot; BIWEEKLY</div>
          <div className="report-window">{metrics.window}</div>
        </div>
        <div className="report-rule" />
        <div className="report-section">
          <div className="report-row">
            <span>captured / wk</span>
            <span className="report-num">{metrics.capturePerWeek}</span>
          </div>
          <div className="report-row">
            <span>return rate</span>
            <span className="report-num">{Math.round(metrics.returnRate * 100)}%</span>
          </div>
          <div className="report-row">
            <span>inbox sanity (median)</span>
            <span className="report-num">{metrics.inboxSanity}</span>
          </div>
          <div className="report-row">
            <span>skipped sundays</span>
            <span className="report-num">{metrics.skippedSundays}</span>
          </div>
        </div>
        <div className="report-rule dotted" />
        <div className="report-section">
          <div className="report-sublabel">capture, daily</div>
          <Spark data={metrics.sparkCapture} color={palette.good} />
          <div className="report-sublabel">complete, daily</div>
          <Spark data={metrics.sparkComplete} color={palette.flow} />
        </div>
        <div className="report-rule dotted" />
        <div className="report-notes">
          <div className="report-note-line">no streaks counted. no penalties applied.</div>
          <div className="report-note-line">nothing here is overdue.</div>
        </div>
        <div className="report-rule" />
        <div className="report-foot">
          <button className="report-x" onClick={onClose}>close</button>
          <span className="report-stamp">RECEIPT &middot; PRINTED 04.26.2026</span>
        </div>
      </div>
    </div>
  );
}

function Spark({ data, color }) {
  const w = 240, h = 26;
  const max = Math.max(1, ...data);
  const step = w / (data.length - 1);
  const path = data.map((v, i) => `${i === 0 ? 'M' : 'L'} ${i * step} ${h - (v / max) * (h - 2) - 1}`).join(' ');
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ margin: '4px 0 8px' }}>
      <path d={path} stroke={color} strokeWidth="1" fill="none" />
      {data.map((v, i) => (
        <circle key={i} cx={i * step} cy={h - (v / max) * (h - 2) - 1} r="1" fill={color} />
      ))}
    </svg>
  );
}

Object.assign(window, { BurndownChart, ContribGraph, VolumeBars, Pile, ReportModal });
