// supply-chain.jsx — grid-based factory flow visual.
//
// Architecture:
//   intake-tank → belt-in (cells) → priority queue (stack) → release belt
//   → workstation (one at a time, shows task title) → belt-out (cells) → drain-tank
//
// Boxes occupy fixed CELLS on belts. They tick step-by-step (with smooth easing
// between cells) and never overlap or pass each other. The priority queue
// accepts boxes and releases the highest-priority one to the workstation.
//
// Modes: stylized | schematic | literal

const { useEffect, useRef, useState, useMemo } = React;

function getPalette(accent) {
  const palettes = {
    teal:   { good: '#3dd6c4', goodDim: '#1a4d49', warn: '#f4b740', warnDim: '#5a3f10', flow: '#3dd6c4', flow2: '#5eb8ff' },
    sodium: { good: '#ffb547', goodDim: '#553a17', warn: '#ff6b3d', warnDim: '#4a1f12', flow: '#ffb547', flow2: '#ff6b3d' },
    cyan:   { good: '#5eb8ff', goodDim: '#173a55', warn: '#f4b740', warnDim: '#5a3f10', flow: '#5eb8ff', flow2: '#3dd6c4' },
    green:  { good: '#5fd97a', goodDim: '#1d4a26', warn: '#f4b740', warnDim: '#5a3f10', flow: '#5fd97a', flow2: '#5eb8ff' }
  };
  return palettes[accent] || palettes.teal;
}

const TAG_HUES = {
  home: 200, work: 30, admin: 300, health: 130, reading: 260,
  errand: 50, idea: 180, reply: 0, fix: 350
};
function tagColor(tag) {
  const h = TAG_HUES[tag] != null ? TAG_HUES[tag] : 220;
  return `oklch(0.66 0.14 ${h})`;
}
function tagColorDim(tag) {
  const h = TAG_HUES[tag] != null ? TAG_HUES[tag] : 220;
  return `oklch(0.32 0.06 ${h})`;
}

// Priority class — drives box body color. 'urgent' (red), 'important' (orange),
// 'normal' (cool blue), 'low' (gray). Determined from age + tag.
function priorityClass(todo) {
  const age = todo.age || 0;
  const tag = todo.tag;
  if (age >= 18) return 'urgent';
  if (age >= 10 && (tag === 'admin' || tag === 'fix' || tag === 'reply')) return 'urgent';
  if (age <= 4 && (tag === 'reply' || tag === 'fix' || tag === 'health' || tag === 'work')) return 'important';
  if (age <= 2) return 'important';
  if (tag === 'reading' || tag === 'idea') return 'low';
  return 'normal';
}
const PRIORITY_PALETTE = {
  urgent:    { body: '#5a1a1c', stripe: '#ff4a4a', glow: 'rgba(255,74,74,0.5)', label: 'URGENT'    },
  important: { body: '#5a3f10', stripe: '#f4b740', glow: 'rgba(244,183,64,0.4)', label: 'IMPORTANT' },
  normal:    { body: '#1f2a35', stripe: '#5eb8ff', glow: 'rgba(94,184,255,0.3)', label: 'NORMAL'    },
  low:       { body: '#22262c', stripe: '#5a6068', glow: 'rgba(120,128,140,0.2)', label: 'LOW'       }
};

// Priority score: lower age = higher priority (use it for ranking).
// Older (stale) items get a small bump downward; tags don't matter for ordering here.
function priorityOf(todo) {
  // Newer items first; tweak so that small-ish ages float up.
  // Bias replies and fixes slightly higher.
  const tagBias = todo.tag === 'reply' ? -1 : todo.tag === 'fix' ? -0.5 : 0;
  return (todo.age || 5) + tagBias;
}

function timeOfDayFactor() {
  const h = new Date().getHours() + new Date().getMinutes() / 60;
  const peak = 11;
  const dist = Math.abs(h - peak);
  const v = Math.exp(-Math.pow(dist / 6, 2)) * 0.9 + 0.4;
  const evening = Math.exp(-Math.pow((h - 20) / 2, 2)) * 0.25;
  return Math.min(1.4, v + evening);
}

// Easing
const easeInOut = (x) => x < 0.5 ? 2*x*x : 1 - Math.pow(-2*x + 2, 2) / 2;

function SupplyChain({ counts, items, intensity, accent, mode, onNodeHover }) {
  const canvasRef = useRef(null);
  const containerRef = useRef(null);
  const [size, setSize] = useState({ w: 1200, h: 480 });
  const palette = getPalette(accent);
  const countsRef = useRef(counts);
  countsRef.current = counts;
  const [tooltip, setTooltip] = useState(null);
  // Mutable map: boxId -> { x, y, sz, box } updated each draw for hit-testing
  const screenMapRef = useRef(new Map());

  // Render-driven state via stateRef. We deliberately avoid useState for hot
  // simulation values to keep the rAF loop allocation-free.
  const stateRef = useRef({
    boxes: [],            // all boxes anywhere in the system
    queue: [],            // boxes parked in priority queue (sorted high→low)
    onStation: null,      // single box currently being worked
    stationProgress: 0,   // 0..1 of work done on current box
    stationStart: 0,      // sim time when station accepted current box
    stationWorkTime: 4.0, // seconds of work per box (modulated by intensity)
    spawnAccum: 0,
    lastT: 0,
    time: 0,
    todFactor: timeOfDayFactor(),
    // Ambient events
    nextShiftChange: 30 + Math.random()*30,
    shiftFlicker: 0,         // 0..1, decays to 0 over 0.6s
    nextInspection: 25 + Math.random()*25,
    inspectionEnd: 0,        // time when current inspection ends
    inspectionLane: 0,       // not used (single lane), placeholder
    inspectionStation: false,
    nextBotPass: 60 + Math.random()*30,
    botX: -1,                // -1 = inactive, otherwise 0..1 along belt
    botSpeed: 0.04,          // per second along belt
    // Gauges/throughput
    intake: 0,
    drain: 0,
    intakeFlash: 0,
    drainFlash: 0,
    pressure: 0,             // 0..1 — how full the queue is
    leds: [0,0,0,0,0,0,0,0],
    ledDirection: 1,
    ledIdx: 0,
    ledClock: 0,
    // Steam puffs from machinery
    puffs: [],
    // Sparks at workstation
    sparks: [],
    // Vibration phase for hum effect
    hum: 0
  });

  // Resize observer
  useEffect(() => {
    if (!containerRef.current) return;
    const ro = new ResizeObserver((entries) => {
      const r = entries[0].contentRect;
      setSize({ w: Math.max(700, r.width), h: 480 });
    });
    ro.observe(containerRef.current);
    return () => ro.disconnect();
  }, []);

  // Layout: belts as cell-grids, machines positioned in flow order.
  const layout = useMemo(() => {
    const W = size.w, H = size.h;
    const cell = 36;          // cell width (px) — boxes step this much
    const beltY = H * 0.62;   // main belt y-coordinate
    const beltH = 28;         // belt thickness

    // Machines
    const intakeW = 86, intakeH = 100;
    const drainW = 86, drainH = 100;
    const queueW = 84, queueH = 200;
    const stationW = 240, stationH = 96;

    // X anchors
    const intakeX = 28;
    const drainX = W - intakeX - drainW;

    const beltInLeftEdge = intakeX + intakeW + 6;
    const queueX = Math.max(beltInLeftEdge + cell*8 + 30, W * 0.30);
    const stationX = W * 0.55;
    const beltOutLeftEdge = stationX + stationW + 16;
    const beltOutRightEdge = drainX - 6;

    // Compute number of cells for each belt segment
    const beltInCells = Math.max(4, Math.floor((queueX - beltInLeftEdge) / cell));
    const beltStnCells = Math.max(2, Math.floor((stationX - (queueX + queueW + 8)) / cell));
    const beltOutCells = Math.max(4, Math.floor((beltOutRightEdge - beltOutLeftEdge) / cell));

    // Define cell coordinates
    const beltIn = Array.from({length: beltInCells}, (_, i) => ({
      x: beltInLeftEdge + i*cell + cell/2, y: beltY
    }));
    const beltStn = Array.from({length: beltStnCells}, (_, i) => ({
      x: queueX + queueW + 8 + i*cell + cell/2, y: beltY
    }));
    const beltOut = Array.from({length: beltOutCells}, (_, i) => ({
      x: beltOutLeftEdge + i*cell + cell/2, y: beltY
    }));

    // Queue slots (vertical stack, top = highest priority)
    const queueSlots = 5;
    const queueSlotH = 28;
    const queueTop = beltY - 14 - (queueSlots - 1) * queueSlotH;
    const queueCellY = beltY; // entry point on the belt y
    const queue = Array.from({length: queueSlots}, (_, i) => ({
      x: queueX + queueW/2,
      y: queueTop + i * queueSlotH
    }));

    // Parked shelf — sits directly above the priority queue, filling the
    // empty band between top strip and queue. Labelled rack with rollover items.
    const parkedSlots = 6;
    const parkedSlotW = 30;
    const parkedTotalW = parkedSlots * parkedSlotW + (parkedSlots - 1) * 4;
    const parkedX = queueX + queueW/2 - parkedTotalW/2;
    const parkedY = queueTop - 70;
    const parked = Array.from({length: parkedSlots}, (_, i) => ({
      x: parkedX + i * (parkedSlotW + 4) + parkedSlotW/2,
      y: parkedY + 18
    }));

    // Rest pile — directly below the intake tank. Fits in the space between
    // intake bottom and the panel floor.
    const restX = intakeX + intakeW/2;
    const restY = H - 36;

    return {
      W, H, cell, beltY, beltH,
      intake: { x: intakeX, y: beltY - intakeH/2 + 30, w: intakeW, h: intakeH },
      drain:  { x: drainX,  y: beltY - drainH/2 + 30,  w: drainW,  h: drainH  },
      queueBox: { x: queueX, y: queueTop - 8, w: queueW, h: queueSlots * queueSlotH + 16 },
      station: { x: stationX, y: beltY - stationH + 18, w: stationW, h: stationH },
      beltIn, beltStn, beltOut, queue, queueSlots,
      parked, parkedSlots, parkedBox: { x: parkedX - 8, y: parkedY, w: parkedTotalW + 16, h: 44 },
      rest: { x: restX, y: restY },
      // Top satellite strip
      stripY: 38
    };
  }, [size]);

  // Maintain hum on every render (for ambient mono number vibration)
  useEffect(() => {
    let raf;
    function loop(t) {
      stateRef.current.hum = (Math.sin(t * 0.008) * 0.5 + Math.sin(t * 0.013) * 0.5);
      raf = requestAnimationFrame(loop);
    }
    raf = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(raf);
  }, []);

  // Main simulation loop
  useEffect(() => {
    const cv = canvasRef.current;
    if (!cv) return;
    const ctx = cv.getContext('2d');
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    cv.width = layout.W * dpr;
    cv.height = layout.H * dpr;
    cv.style.width = layout.W + 'px';
    cv.style.height = layout.H + 'px';
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    const st = stateRef.current;
    let raf;
    let lastReal = performance.now();

    // Build a "live items pool" — we'll mostly use 'this-week' status as the
    // active queue feeding into the workstation. But ambient flow uses inbox
    // → queue when this-week is empty.
    function activeItems() {
      if (!Array.isArray(items)) return [];
      const ranked = items
        .filter(it => it.status === 'this-week' || it.status === 'inbox')
        .map(it => ({ ...it, _p: priorityOf(it) }));
      ranked.sort((a, b) => a._p - b._p);
      return ranked;
    }

    // Spawn a new box at intake. Pulls a real item if available; falls back to
    // a placeholder so the simulation never stalls visually.
    function spawnBox() {
      const items = activeItems();
      let title, tag, age, klass;
      if (items.length > 0) {
        // Weight pick toward higher-priority classes so the user sees urgent
        // items show up at the workstation more often.
        const weights = items.map(it => {
          const k = priorityClass(it);
          return k === 'urgent' ? 5 : k === 'important' ? 3 : k === 'normal' ? 2 : 1;
        });
        const total = weights.reduce((a,b) => a + b, 0);
        let r = Math.random() * total, idx = 0;
        for (let i = 0; i < weights.length; i++) {
          r -= weights[i];
          if (r <= 0) { idx = i; break; }
        }
        const it = items[idx];
        title = it.t; tag = it.tag; age = it.age;
        klass = priorityClass(it);
      } else {
        title = 'pending capture'; tag = 'idea'; age = 1; klass = 'low';
      }
      const box = {
        id: 'b' + Math.random().toString(36).slice(2,8),
        title, tag, age, klass,
        priority: priorityOf({tag, age}),
        // location state machine
        loc: 'beltIn', cell: 0, // current cell index (integer)
        targetCell: 0,
        cellProgress: 1,        // 0..1 between previous and target cell
        stepDuration: 0.35,     // seconds per cell step
        stamped: false,
        stallPulse: 0,          // 0..1 visual when stalled
        // for queue rendering
        queueSlot: -1,
        // for station rendering
        bornAt: st.time
      };
      st.boxes.push(box);
    }

    function step(dt) {
      st.time += dt;
      const I = (intensity || 60) / 100;
      const tod = st.todFactor;

      // ----- Spawn cadence -----
      // Step duration is fixed; we control how fast new boxes enter.
      const spawnHz = 0.35 * I * tod;  // boxes/sec
      st.spawnAccum += dt * spawnHz;
      while (st.spawnAccum >= 1) {
        st.spawnAccum -= 1;
        // Only spawn if first-cell of beltIn is free
        const occ = st.boxes.find(b => b.loc === 'beltIn' && b.cell === 0 && b.cellProgress >= 0.85);
        if (!occ) {
          spawnBox();
          st.intakeFlash = 1;
          st.intake = st.intake * 0.85 + 1;
        }
      }
      st.intake *= Math.exp(-dt * 0.4);
      st.drain *= Math.exp(-dt * 0.4);
      st.intakeFlash = Math.max(0, st.intakeFlash - dt * 2);
      st.drainFlash = Math.max(0, st.drainFlash - dt * 2);

      // ----- Belt step logic -----
      // Group boxes by belt for easy per-cell occupancy lookup.
      // For each box in flight, check if next cell is free; if so, advance.

      // Compute occupancy maps
      const occ = { beltIn: new Set(), beltStn: new Set(), beltOut: new Set() };
      for (const b of st.boxes) {
        if (b.loc in occ) {
          // While moving (progress < 1), it occupies BOTH source and target cells
          occ[b.loc].add(b.cell);
        }
      }

      // Sort boxes by location-priority so leaders move first.
      // For belts: highest cell first (downstream first). For queue: priority order.
      const boxesByLoc = {
        beltIn: [], beltStn: [], beltOut: [], queue: [], station: [], drained: []
      };
      for (const b of st.boxes) (boxesByLoc[b.loc] || boxesByLoc.beltIn).push(b);
      boxesByLoc.beltIn.sort((a,b) => b.cell - a.cell);
      boxesByLoc.beltStn.sort((a,b) => b.cell - a.cell);
      boxesByLoc.beltOut.sort((a,b) => b.cell - a.cell);

      // ---- BELT IN ----
      // Boxes step from cell 0 → last cell, then try to enter QUEUE.
      // If queue full: stall on the last cell.
      for (const b of boxesByLoc.beltIn) {
        if (b.cellProgress < 1) {
          b.cellProgress = Math.min(1, b.cellProgress + dt / b.stepDuration);
          if (b.cellProgress >= 1) {
            // landed in target cell
          }
          continue;
        }
        // At rest in cell `b.cell`. Try to advance.
        const lastIdx = layout.beltIn.length - 1;
        if (b.cell < lastIdx) {
          const next = b.cell + 1;
          if (!occ.beltIn.has(next)) {
            occ.beltIn.delete(b.cell);
            occ.beltIn.add(next);
            b.cell = next;
            b.cellProgress = 0;
            b.stepDuration = 0.35 / Math.max(0.4, I * tod);
            b.stallPulse = 0;
          } else {
            b.stallPulse = Math.min(1, b.stallPulse + dt * 2);
          }
        } else {
          // At end of belt-in; try to enter the queue
          const slot = findFreeQueueSlot();
          if (slot >= 0) {
            // transfer
            occ.beltIn.delete(b.cell);
            b.loc = 'queue';
            b.queueSlot = slot;
            b.cellProgress = 0;             // animate move into slot
            b.stepDuration = 0.4;
            st.queue.push(b);
            // re-sort queue by priority asc (lower priority value = higher)
            sortQueue();
          } else {
            b.stallPulse = Math.min(1, b.stallPulse + dt * 2);
          }
        }
      }

      // ---- QUEUE ----
      // Boxes ease into their assigned slot, then sit until released.
      // Highest-priority (slot 0) releases when station free + belt-stn cell 0 free.
      for (const b of st.queue) {
        if (b.cellProgress < 1) {
          b.cellProgress = Math.min(1, b.cellProgress + dt / b.stepDuration);
        }
      }
      // Try release
      if (!st.onStation && st.queue.length > 0) {
        const head = st.queue[0];
        if (head.cellProgress >= 1) {
          // Belt-stn cell 0 free?
          if (!occ.beltStn.has(0) && layout.beltStn.length > 0) {
            // Move from queue → beltStn cell 0
            st.queue.shift();
            // Re-sort + reassign slots for remaining queue
            sortQueue();
            head.loc = 'beltStn';
            head.cell = 0;
            head.cellProgress = 0;
            head.stepDuration = 0.35 / Math.max(0.4, I * tod);
            head.queueSlot = -1;
            occ.beltStn.add(0);
          }
        }
      }

      // ---- BELT STN (queue → workstation) ----
      for (const b of boxesByLoc.beltStn) {
        if (b.cellProgress < 1) {
          b.cellProgress = Math.min(1, b.cellProgress + dt / b.stepDuration);
          continue;
        }
        const lastIdx = layout.beltStn.length - 1;
        if (b.cell < lastIdx) {
          const next = b.cell + 1;
          if (!occ.beltStn.has(next)) {
            occ.beltStn.delete(b.cell);
            occ.beltStn.add(next);
            b.cell = next;
            b.cellProgress = 0;
            b.stepDuration = 0.35 / Math.max(0.4, I * tod);
          }
        } else {
          // Try to enter station
          if (!st.onStation) {
            occ.beltStn.delete(b.cell);
            b.loc = 'station';
            b.cell = 0;
            b.cellProgress = 0;
            b.stepDuration = 0.55;
            st.onStation = b;
            st.stationProgress = 0;
            st.stationStart = st.time;
          }
        }
      }

      // ---- STATION (one box at a time) ----
      if (st.onStation) {
        const b = st.onStation;
        if (b.cellProgress < 1) {
          b.cellProgress = Math.min(1, b.cellProgress + dt / b.stepDuration);
        } else {
          // Box has arrived at the station — wait for user to press DONE.
          // No automatic progress; sparks only flicker subtly to show it's "live".
          if (mode !== 'literal' && Math.random() < dt * 1.4) {
            st.sparks.push({
              x: layout.station.x + layout.station.w * 0.5 + (Math.random()-.5)*30,
              y: layout.station.y + 20 + Math.random()*8,
              vx: (Math.random()-.5)*40, vy: -30 - Math.random()*30,
              life: 0.5 + Math.random()*0.4, age: 0
            });
          }
          if (st.completeRequested) {
            // User pressed DONE
            const cellFree = !occ.beltOut.has(0);
            if (cellFree && layout.beltOut.length > 0) {
              b.stamped = true;
              b.loc = 'beltOut';
              b.cell = 0;
              b.cellProgress = 0;
              b.stepDuration = 0.32 / Math.max(0.4, I * tod);
              occ.beltOut.add(0);
              st.onStation = null;
              st.stationProgress = 0;
              st.completeRequested = false;
              st.drainFlash = 0;
              st.puffs.push({
                x: layout.station.x + layout.station.w/2,
                y: layout.station.y - 4,
                age: 0, life: 0.9, vy: -22
              });
            }
          }
        }
      }

      // ---- BELT OUT ----
      for (const b of boxesByLoc.beltOut) {
        if (b.cellProgress < 1) {
          b.cellProgress = Math.min(1, b.cellProgress + dt / b.stepDuration);
          continue;
        }
        const lastIdx = layout.beltOut.length - 1;
        if (b.cell < lastIdx) {
          const next = b.cell + 1;
          if (!occ.beltOut.has(next)) {
            occ.beltOut.delete(b.cell);
            occ.beltOut.add(next);
            b.cell = next;
            b.cellProgress = 0;
            b.stepDuration = 0.32 / Math.max(0.4, I * tod);
          }
        } else {
          // Drain — remove from sim
          occ.beltOut.delete(b.cell);
          b.loc = 'drained';
          st.drainFlash = 1;
          st.drain = st.drain * 0.85 + 1;
        }
      }

      // Cleanup drained boxes
      st.boxes = st.boxes.filter(b => b.loc !== 'drained');

      // Pressure = queue fullness
      st.pressure = st.queue.length / layout.queueSlots;

      // ---- Sparks ----
      for (const s of st.sparks) {
        s.age += dt;
        s.x += s.vx * dt;
        s.y += s.vy * dt;
        s.vy += 60 * dt; // gravity
      }
      st.sparks = st.sparks.filter(s => s.age < s.life);

      // ---- Steam puffs ----
      // Add periodic puffs from intake + drain machinery
      if (Math.random() < dt * 0.4) {
        st.puffs.push({
          x: layout.intake.x + layout.intake.w * 0.5,
          y: layout.intake.y + 6,
          age: 0, life: 1.2, vy: -16 - Math.random()*10
        });
      }
      if (Math.random() < dt * 0.35) {
        st.puffs.push({
          x: layout.drain.x + layout.drain.w * 0.5,
          y: layout.drain.y + 6,
          age: 0, life: 1.2, vy: -16 - Math.random()*10
        });
      }
      for (const p of st.puffs) {
        p.age += dt;
        p.y += p.vy * dt;
        p.vy += 4 * dt; // drift slows
      }
      st.puffs = st.puffs.filter(p => p.age < p.life);

      // ---- LEDs ----
      st.ledClock += dt;
      if (st.ledClock > 0.18) {
        st.ledClock = 0;
        for (let i = 0; i < st.leds.length; i++) st.leds[i] *= 0.55;
        st.leds[st.ledIdx] = 1;
        st.ledIdx += st.ledDirection;
        if (st.ledIdx >= st.leds.length - 1) { st.ledIdx = st.leds.length - 1; st.ledDirection = -1; }
        if (st.ledIdx <= 0) { st.ledIdx = 0; st.ledDirection = 1; }
      }

      // ---- Ambient: shift change ----
      st.nextShiftChange -= dt;
      if (st.nextShiftChange <= 0) {
        st.shiftFlicker = 1;
        st.nextShiftChange = 50 + Math.random()*40;
      }
      st.shiftFlicker = Math.max(0, st.shiftFlicker - dt * 1.6);

      // ---- Ambient: inspection ----
      st.nextInspection -= dt;
      if (st.nextInspection <= 0 && !st.inspectionStation) {
        st.inspectionStation = true;
        st.inspectionEnd = st.time + 3.5;
        st.nextInspection = 35 + Math.random()*30;
      }
      if (st.inspectionStation && st.time >= st.inspectionEnd) {
        st.inspectionStation = false;
      }

      // ---- Ambient: maintenance bot ----
      st.nextBotPass -= dt;
      if (st.nextBotPass <= 0 && st.botX < 0) {
        st.botX = 0;
        st.botSpeed = 0.06 + Math.random()*0.04;
        st.nextBotPass = 90 + Math.random()*40;
      }
      if (st.botX >= 0) {
        st.botX += st.botSpeed * dt;
        if (st.botX > 1.05) st.botX = -1;
      }

      // ---- TOD ----
      st.todFactor = st.todFactor * 0.99 + timeOfDayFactor() * 0.01;
    }

    function findFreeQueueSlot() {
      // Find first empty slot index (0..queueSlots-1) by checking st.queue length
      if (st.queue.length < layout.queueSlots) return st.queue.length;
      return -1;
    }

    function sortQueue() {
      st.queue.sort((a, b) => a.priority - b.priority);
      // Reassign slots top-down
      st.queue.forEach((b, i) => {
        if (b.queueSlot !== i) {
          b.queueSlot = i;
          // animate to new slot
          b.cellProgress = Math.min(b.cellProgress, 0.5);
        }
      });
    }

    // ---------------------- DRAW ----------------------
    function draw() {
      const W = layout.W, H = layout.H;
      // bg
      ctx.fillStyle = '#050608';
      ctx.fillRect(0, 0, W, H);

      // Faint scan / vignette
      if (mode !== 'literal') {
        ctx.fillStyle = 'rgba(255,255,255,0.012)';
        ctx.fillRect(0, 0, W, 12);
      }

      // Shift flicker
      if (st.shiftFlicker > 0.05) {
        ctx.fillStyle = `rgba(0,0,0,${0.3 * st.shiftFlicker * (Math.random()*0.6 + 0.4)})`;
        ctx.fillRect(0, 0, W, H);
      }

      drawTopStrip(ctx, W, H, layout, st, palette, mode);
      drawIntake(ctx, layout.intake, palette, st, mode);
      drawDrain(ctx, layout.drain, palette, st, mode);
      drawBelt(ctx, layout.beltIn, layout.cell, layout.beltH, palette, mode, st, 'in');
      drawBelt(ctx, layout.beltStn, layout.cell, layout.beltH, palette, mode, st, 'stn');
      drawBelt(ctx, layout.beltOut, layout.cell, layout.beltH, palette, mode, st, 'out');
      drawQueue(ctx, layout.queueBox, layout.queue, st, palette, mode);
      drawStation(ctx, layout.station, st, palette, mode);
      drawParked(ctx, layout, st, countsRef.current, palette, mode);
      drawRest(ctx, layout, st, countsRef.current, palette, mode);

      // Boxes
      const sm = screenMapRef.current;
      sm.clear();
      for (const b of st.boxes) drawBox(ctx, b, layout, palette, mode, sm);
      if (st.onStation) drawBox(ctx, st.onStation, layout, palette, mode, sm);

      // Maintenance bot (drawn over belts)
      if (st.botX >= 0) drawBot(ctx, st, layout, palette);

      // Sparks
      for (const s of st.sparks) {
        const a = 1 - s.age / s.life;
        ctx.globalAlpha = a;
        ctx.fillStyle = palette.warn;
        ctx.fillRect(s.x, s.y, 1.5, 1.5);
      }
      ctx.globalAlpha = 1;

      // Steam puffs
      for (const p of st.puffs) {
        const a = 0.5 * (1 - p.age / p.life);
        const r = 4 + p.age * 14;
        ctx.globalAlpha = a;
        ctx.fillStyle = '#cccccc';
        ctx.beginPath(); ctx.arc(p.x, p.y, r, 0, Math.PI*2); ctx.fill();
      }
      ctx.globalAlpha = 1;

      // Inspection flag at station
      if (st.inspectionStation) drawInspectionFlag(ctx, layout.station, st);
    }

    let prev = performance.now();
    function tick(now) {
      const dt = Math.min(0.05, (now - prev) / 1000);
      prev = now;
      step(dt);
      draw();
      raf = requestAnimationFrame(tick);
    }
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [layout, palette, mode, intensity, items]);

  function handleMouseMove(e) {
    const rect = e.currentTarget.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    // Check DONE button hover first
    const btn = stateRef.current && stateRef.current._doneBtn;
    let overBtn = false;
    if (btn && mx >= btn.x && mx <= btn.x + btn.w && my >= btn.y && my <= btn.y + btn.h) {
      overBtn = true;
    }
    let hit = null, hitDist = 16;
    if (!overBtn) {
      for (const v of screenMapRef.current.values()) {
        const dx = mx - v.x, dy = my - v.y;
        const half = v.sz / 2 + 2;
        if (Math.abs(dx) <= half && Math.abs(dy) <= half) {
          const d = Math.abs(dx) + Math.abs(dy);
          if (d < hitDist) { hitDist = d; hit = v; }
        }
      }
    }
    if (hit) {
      setTooltip({ x: mx, y: my, box: hit.box });
    } else if (tooltip) {
      setTooltip(null);
    }
    e.currentTarget.style.cursor = overBtn ? 'pointer' : (hit ? 'help' : 'default');
  }
  function handleMouseLeave() { setTooltip(null); }
  function handleClick(e) {
    const rect = e.currentTarget.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    const btn = stateRef.current && stateRef.current._doneBtn;
    if (btn && mx >= btn.x && mx <= btn.x + btn.w && my >= btn.y && my <= btn.y + btn.h) {
      stateRef.current.completeRequested = true;
    }
  }

  return (
    <div ref={containerRef} className="sc-wrap" style={{position:'relative', width:'100%', height: 480}}>
      <canvas
        ref={canvasRef}
        style={{display:'block', width:'100%', height:'100%'}}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        onClick={handleClick}
      />
      {tooltip && <BoxTooltip x={tooltip.x} y={tooltip.y} box={tooltip.box} />}
    </div>
  );
}

function BoxTooltip({ x, y, box }) {
  const klass = box.klass || 'normal';
  const pp = PRIORITY_PALETTE[klass];
  const left = Math.min(x + 14, 1100);
  const top = Math.max(y - 8, 8);
  return (
    <div className="sc-tip" style={{
      position: 'absolute', left, top, transform: 'translateY(-100%)',
      background: '#0a0c10', border: '1px solid ' + pp.stripe,
      padding: '7px 10px 8px', minWidth: 180, maxWidth: 280,
      fontFamily: 'ui-monospace, monospace', fontSize: 11, color: '#e6e9ef',
      pointerEvents: 'none', zIndex: 20,
      boxShadow: '0 6px 20px rgba(0,0,0,0.5)'
    }}>
      <div style={{display:'flex', alignItems:'center', gap:6, marginBottom:4}}>
        <span style={{
          display:'inline-block', width:8, height:8, background: pp.stripe
        }} />
        <span style={{fontSize:9, letterSpacing:'0.12em', color: pp.stripe, fontWeight:700}}>
          {pp.label}
        </span>
        <span style={{flex:1}} />
        <span style={{fontSize:9, color:'#7a8290'}}>AGE {box.age}d</span>
      </div>
      <div style={{fontSize:12, lineHeight:1.35, color:'#fff', marginBottom:5, textWrap:'pretty'}}>
        {box.title}
      </div>
      <div style={{display:'flex', alignItems:'center', gap:6, fontSize:9, color:'#7a8290'}}>
        <span style={{
          display:'inline-block', width:6, height:6, background: 'oklch(0.66 0.14 ' + (TAG_HUES[box.tag] || 220) + ')'
        }} />
        <span>#{box.tag}</span>
        <span style={{flex:1}} />
        <span>id {box.id}</span>
      </div>
    </div>
  );
}

// ---------------------- DRAW HELPERS ----------------------

function drawTopStrip(ctx, W, H, layout, st, palette, mode) {
  const y = layout.stripY;
  ctx.font = '9px ui-monospace, monospace';
  ctx.textBaseline = 'middle';
  // Left: intake gauge
  drawGauge(ctx, 70, y, 'INTAKE / HR', Math.min(60, st.intake * 12), palette.good, st);
  // Right: drain gauge
  drawGauge(ctx, W - 70, y, 'DRAIN / HR', Math.min(60, st.drain * 12), palette.good, st);
  // Center: time of day + status leds + pressure
  const cx = W / 2;
  const todW = 110, ledsW = 200, prsW = 110, gap = 24;
  const total = todW + ledsW + prsW + gap*2;
  const startX = cx - total/2;
  drawTOD(ctx, startX + todW/2, y, st);
  drawLEDs(ctx, startX + todW + gap + ledsW/2, y - 8, st, palette);
  drawPressure(ctx, startX + todW + gap + ledsW + gap + prsW/2, y, st, palette);
}

function drawGauge(ctx, cx, cy, label, value, color, st) {
  const r = 22;
  // dial
  ctx.strokeStyle = '#1c1f23';
  ctx.lineWidth = 4;
  ctx.beginPath(); ctx.arc(cx, cy, r, Math.PI*0.75, Math.PI*0.25, false); ctx.stroke();
  // value arc
  const f = Math.max(0, Math.min(1, value/60));
  const a0 = Math.PI*0.75;
  const a1 = a0 + Math.PI*1.5*f;
  ctx.strokeStyle = color;
  ctx.lineWidth = 4;
  ctx.beginPath(); ctx.arc(cx, cy, r, a0, a1, false); ctx.stroke();
  // needle
  const ang = a1;
  ctx.strokeStyle = '#dadcdf';
  ctx.lineWidth = 1.5;
  ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(cx + Math.cos(ang)*(r-4), cy + Math.sin(ang)*(r-4)); ctx.stroke();
  // hub
  ctx.fillStyle = '#0a0c0f'; ctx.beginPath(); ctx.arc(cx, cy, 2.5, 0, Math.PI*2); ctx.fill();
  // value text (vibrating slightly with hum)
  ctx.fillStyle = '#dadcdf';
  ctx.textAlign = 'center';
  const jitterY = st.hum * 0.4;
  ctx.font = '11px ui-monospace, monospace';
  ctx.fillText(Math.round(value), cx, cy + r + 11 + jitterY);
  // label
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace';
  ctx.fillText(label, cx, cy - r - 8);
}

function drawTOD(ctx, cx, cy, st) {
  const w = 110, h = 28;
  const x = cx - w/2, y = cy - h/2;
  // border
  ctx.strokeStyle = '#1c1f23'; ctx.lineWidth = 1;
  ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  // bar
  const f = Math.min(1, (st.todFactor - 0.4) / 1.0);
  ctx.fillStyle = '#1a4d49';
  ctx.fillRect(x+3, y+3, (w-6)*f, h-6);
  // tick marks
  ctx.fillStyle = '#2a2e34';
  for (let i = 1; i < 6; i++) {
    ctx.fillRect(x + i*(w/6), y+2, 1, h-4);
  }
  // label
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace';
  ctx.textAlign = 'center';
  ctx.fillText('TIME-OF-DAY', cx, y - 5);
  // value
  ctx.fillStyle = '#dadcdf';
  ctx.font = '9px ui-monospace, monospace';
  ctx.textAlign = 'right';
  ctx.fillText(st.todFactor.toFixed(2) + 'x', x + w - 4, cy);
}

function drawLEDs(ctx, cx, cy, st, palette) {
  const n = st.leds.length, gap = 4, sz = 16;
  const total = n * sz + (n-1)*gap;
  const startX = cx - total/2;
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace'; ctx.textAlign = 'center';
  ctx.fillText('STATUS', cx, cy - 11);
  for (let i = 0; i < n; i++) {
    const x = startX + i * (sz + gap);
    const v = st.leds[i];
    ctx.fillStyle = '#0d0f12';
    ctx.fillRect(x, cy - 4, sz, 8);
    ctx.fillStyle = `rgba(61,214,196,${0.15 + v * 0.85})`;
    ctx.fillRect(x+1, cy - 3, sz-2, 6);
    if (v > 0.5) {
      ctx.fillStyle = `rgba(61,214,196,${(v-0.5)*0.6})`;
      ctx.fillRect(x-2, cy - 6, sz+4, 12);
    }
  }
}

function drawPressure(ctx, cx, cy, st, palette) {
  const w = 110, h = 28;
  const x = cx - w/2, y = cy - h/2;
  ctx.strokeStyle = '#1c1f23'; ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  // segmented fill
  const f = st.pressure;
  const segs = 10;
  for (let i = 0; i < segs; i++) {
    const sx = x + 3 + i * ((w-6)/segs);
    const lit = (i+0.5)/segs <= f;
    const high = i >= segs * 0.7;
    ctx.fillStyle = lit ? (high ? palette.warn : palette.good) : '#1c1f23';
    ctx.fillRect(sx, y+5, (w-6)/segs - 1.5, h-10);
  }
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace'; ctx.textAlign = 'center';
  ctx.fillText('QUEUE PRESSURE', cx, y - 5);
}

function drawIntake(ctx, n, palette, st, mode) {
  const x = n.x, y = n.y, w = n.w, h = n.h;
  // tank shell
  ctx.strokeStyle = '#1c1f23'; ctx.lineWidth = 1;
  ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  // bolts
  ctx.fillStyle = '#1c1f23';
  [[4,4],[w-4,4],[4,h-4],[w-4,h-4]].forEach(([dx,dy]) => {
    ctx.beginPath(); ctx.arc(x+dx, y+dy, 1.5, 0, Math.PI*2); ctx.fill();
  });
  // liquid (rises with intake)
  const fillH = Math.min(h-12, 30 + st.intake * 8);
  ctx.fillStyle = palette.flow;
  ctx.globalAlpha = 0.18;
  ctx.fillRect(x+5, y+h - 5 - fillH, w-10, fillH);
  ctx.globalAlpha = 1;
  // surface
  ctx.strokeStyle = palette.flow;
  ctx.lineWidth = 1;
  ctx.beginPath(); ctx.moveTo(x+5, y+h-5-fillH); ctx.lineTo(x+w-5, y+h-5-fillH); ctx.stroke();
  // label
  ctx.fillStyle = '#5a6068';
  ctx.font = '9px ui-monospace, monospace'; ctx.textAlign = 'center';
  ctx.fillText('INTAKE', x + w/2, y - 6);
  ctx.fillText('inbox', x + w/2, y + h + 12);
  // flash
  if (st.intakeFlash > 0) {
    ctx.strokeStyle = palette.flow;
    ctx.globalAlpha = st.intakeFlash;
    ctx.strokeRect(x-1.5, y-1.5, w+3, h+3);
    ctx.globalAlpha = 1;
  }
  // exit pipe stub
  ctx.fillStyle = '#0d0f12';
  ctx.fillRect(x + w, y + h*0.55 - 6, 8, 12);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x + w + 0.5, y + h*0.55 - 6 + 0.5, 8 - 1, 12 - 1);
}

function drawDrain(ctx, n, palette, st, mode) {
  const x = n.x, y = n.y, w = n.w, h = n.h;
  ctx.strokeStyle = '#1c1f23'; ctx.lineWidth = 1;
  ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  ctx.fillStyle = '#1c1f23';
  [[4,4],[w-4,4],[4,h-4],[w-4,h-4]].forEach(([dx,dy]) => {
    ctx.beginPath(); ctx.arc(x+dx, y+dy, 1.5, 0, Math.PI*2); ctx.fill();
  });
  // drain liquid
  const fillH = Math.min(h-12, 26 + st.drain * 8);
  ctx.fillStyle = palette.good;
  ctx.globalAlpha = 0.22;
  ctx.fillRect(x+5, y+h-5-fillH, w-10, fillH);
  ctx.globalAlpha = 1;
  ctx.strokeStyle = palette.good;
  ctx.lineWidth = 1;
  ctx.beginPath(); ctx.moveTo(x+5, y+h-5-fillH); ctx.lineTo(x+w-5, y+h-5-fillH); ctx.stroke();
  ctx.fillStyle = '#5a6068';
  ctx.font = '9px ui-monospace, monospace'; ctx.textAlign = 'center';
  ctx.fillText('DRAIN', x + w/2, y - 6);
  ctx.fillText('shipped', x + w/2, y + h + 12);
  if (st.drainFlash > 0) {
    ctx.strokeStyle = palette.good;
    ctx.globalAlpha = st.drainFlash;
    ctx.strokeRect(x-1.5, y-1.5, w+3, h+3);
    ctx.globalAlpha = 1;
  }
  // entry pipe stub
  ctx.fillStyle = '#0d0f12';
  ctx.fillRect(x - 8, y + h*0.55 - 6, 8, 12);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x - 8 + 0.5, y + h*0.55 - 6 + 0.5, 8 - 1, 12 - 1);
}

function drawBelt(ctx, cells, cell, beltH, palette, mode, st, key) {
  if (!cells.length) return;
  const x0 = cells[0].x - cell/2;
  const x1 = cells[cells.length-1].x + cell/2;
  const y = cells[0].y;
  // belt body
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(x0, y - beltH/2, x1 - x0, beltH);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x0+0.5, y - beltH/2 + 0.5, x1 - x0 - 1, beltH - 1);
  // tread chevrons (animated)
  const phase = (st.time * 16) % cell;
  ctx.strokeStyle = '#16191d';
  ctx.lineWidth = 1.5;
  for (let xx = x0 + (phase % cell); xx < x1; xx += cell/2) {
    ctx.beginPath();
    ctx.moveTo(xx - 4, y - 4);
    ctx.lineTo(xx, y);
    ctx.lineTo(xx - 4, y + 4);
    ctx.stroke();
  }
  // end rollers
  ctx.fillStyle = '#16191d';
  ctx.beginPath(); ctx.arc(x0, y, beltH/2 - 1, 0, Math.PI*2); ctx.fill();
  ctx.beginPath(); ctx.arc(x1, y, beltH/2 - 1, 0, Math.PI*2); ctx.fill();
  ctx.strokeStyle = '#2a2e34'; ctx.lineWidth = 1;
  ctx.beginPath(); ctx.arc(x0, y, beltH/2 - 1, 0, Math.PI*2); ctx.stroke();
  ctx.beginPath(); ctx.arc(x1, y, beltH/2 - 1, 0, Math.PI*2); ctx.stroke();
}

function drawQueue(ctx, n, slots, st, palette, mode) {
  const { x, y, w, h } = n;
  // shell
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(x, y, w, h);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  // priority gradient bar on left
  const grad = ctx.createLinearGradient(0, y, 0, y + h);
  grad.addColorStop(0, palette.warn);
  grad.addColorStop(1, palette.goodDim);
  ctx.fillStyle = grad;
  ctx.globalAlpha = 0.45;
  ctx.fillRect(x + 2, y + 4, 3, h - 8);
  ctx.globalAlpha = 1;
  // slot ticks
  ctx.strokeStyle = '#15181b';
  for (let i = 0; i < slots.length; i++) {
    const sy = slots[i].y;
    ctx.beginPath();
    ctx.moveTo(x + 8, sy + 14);
    ctx.lineTo(x + w - 4, sy + 14);
    ctx.stroke();
  }
  // label
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace'; ctx.textAlign = 'center';
  ctx.fillText('PRIORITY · QUEUE', x + w/2, y - 6);
  ctx.fillText('top = next', x + w/2, y + h + 11);

  // Queue counter (when 2+)
  if (st.queue.length >= 2) {
    ctx.fillStyle = palette.warn;
    ctx.font = 'bold 11px ui-monospace, monospace';
    ctx.textAlign = 'left';
    ctx.fillText(`+${st.queue.length}`, x + w + 6, y + 12);
  }
}

function drawStation(ctx, n, st, palette, mode) {
  const { x, y, w, h } = n;
  // shell
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(x, y, w, h);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x+0.5, y+0.5, w-1, h-1);
  // top label band
  ctx.fillStyle = '#0d0f12';
  ctx.fillRect(x, y, w, 18);
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace'; ctx.textAlign = 'left';
  ctx.fillText('WORKSTATION · NOW SERVING', x + 6, y + 9);
  // status dot
  const live = !!st.onStation;
  ctx.fillStyle = live ? palette.good : '#2a2e34';
  ctx.beginPath(); ctx.arc(x + w - 8, y + 9, 3, 0, Math.PI*2); ctx.fill();
  if (live) {
    ctx.fillStyle = palette.good;
    ctx.globalAlpha = 0.3 + 0.3 * Math.sin(st.time * 6);
    ctx.beginPath(); ctx.arc(x + w - 8, y + 9, 5, 0, Math.PI*2); ctx.fill();
    ctx.globalAlpha = 1;
  }

  // Pad rectangle (the work area)
  const padY = y + 24, padH = h - 30;
  ctx.strokeStyle = '#15181b';
  ctx.setLineDash([3, 4]);
  ctx.strokeRect(x + 8, padY, w - 16, padH);
  ctx.setLineDash([]);

  // Task title — biggest piece of info
  if (st.onStation) {
    const b = st.onStation;
    // Box visual area is left half; title goes right side
    const titleX = x + 80;
    const titleY = y + 56;
    ctx.fillStyle = '#dadcdf';
    ctx.font = '600 13px ui-sans-serif, system-ui, sans-serif';
    ctx.textAlign = 'left';
    let title = b.title;
    // truncate to fit
    const maxW = w - 96;
    if (ctx.measureText(title).width > maxW) {
      while (title.length > 4 && ctx.measureText(title + '…').width > maxW) {
        title = title.slice(0, -1);
      }
      title = title + '…';
    }
    ctx.fillText(title, titleX, titleY);
    // subline: tag only
    ctx.fillStyle = '#5a6068';
    ctx.font = '9px ui-monospace, monospace';
    ctx.fillText(b.tag.toUpperCase() + ' · WAITING ON YOU', titleX, titleY + 14);
    // tag swatch
    ctx.fillStyle = tagColor(b.tag);
    ctx.fillRect(titleX, titleY + 22, 24, 3);

    // DONE button (bottom-right of pad)
    const onlyArrived = b.cellProgress >= 1;
    const btnW = 78, btnH = 22;
    const btnX = x + w - 12 - btnW;
    const btnY = padY + padH - btnH - 4;
    if (onlyArrived) {
      const pulse = 0.55 + 0.45 * Math.sin(performance.now() * 0.005);
      ctx.fillStyle = '#1d3a24';
      ctx.fillRect(btnX, btnY, btnW, btnH);
      ctx.fillStyle = `rgba(74,222,128,${0.85 * pulse + 0.15})`;
      ctx.fillRect(btnX, btnY, btnW, 2);
      ctx.strokeStyle = palette.good;
      ctx.lineWidth = 1;
      ctx.strokeRect(btnX + 0.5, btnY + 0.5, btnW - 1, btnH - 1);
      // checkmark
      ctx.strokeStyle = palette.good;
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(btnX + 10, btnY + btnH/2);
      ctx.lineTo(btnX + 14, btnY + btnH - 6);
      ctx.lineTo(btnX + 22, btnY + 6);
      ctx.stroke();
      ctx.lineWidth = 1;
      // label
      ctx.fillStyle = palette.good;
      ctx.font = '600 11px ui-monospace, monospace';
      ctx.textAlign = 'left';
      ctx.fillText('MARK DONE', btnX + 28, btnY + btnH/2 + 4);
      // expose for hit-testing
      st._doneBtn = { x: btnX, y: btnY, w: btnW, h: btnH };
    } else {
      // box still arriving — show ghost button
      ctx.strokeStyle = '#2a2e34';
      ctx.setLineDash([2,3]);
      ctx.strokeRect(btnX + 0.5, btnY + 0.5, btnW - 1, btnH - 1);
      ctx.setLineDash([]);
      ctx.fillStyle = '#3a3e44';
      ctx.font = '10px ui-monospace, monospace';
      ctx.textAlign = 'center';
      ctx.fillText('arriving…', btnX + btnW/2, btnY + btnH/2 + 3);
      st._doneBtn = null;
    }
  } else {
    st._doneBtn = null;
    // Idle message
    ctx.fillStyle = '#3a3e44';
    ctx.font = '11px ui-monospace, monospace';
    ctx.textAlign = 'center';
    ctx.fillText('— idle — awaiting next item —', x + w/2, y + h/2 + 6);
  }
}

function drawBox(ctx, b, layout, palette, mode, screenMap) {
  // Resolve current screen position based on loc
  let px, py;
  if (b.loc === 'beltIn' || b.loc === 'beltStn' || b.loc === 'beltOut') {
    const arr = b.loc === 'beltIn' ? layout.beltIn : b.loc === 'beltStn' ? layout.beltStn : layout.beltOut;
    if (b.cellProgress >= 1) {
      px = arr[b.cell].x; py = arr[b.cell].y;
    } else {
      const prev = arr[Math.max(0, b.cell - 1)];
      const cur = arr[b.cell];
      const k = easeInOut(b.cellProgress);
      px = prev.x + (cur.x - prev.x) * k;
      py = prev.y + (cur.y - prev.y) * k;
    }
  } else if (b.loc === 'queue') {
    const slot = layout.queue[b.queueSlot] || layout.queue[0];
    if (b.cellProgress >= 1) {
      px = slot.x; py = slot.y;
    } else {
      // animate from belt-in last cell to slot
      const enter = layout.beltIn[layout.beltIn.length - 1];
      const k = easeInOut(b.cellProgress);
      px = enter.x + (slot.x - enter.x) * k;
      py = enter.y + (slot.y - enter.y) * k;
    }
  } else if (b.loc === 'station') {
    // Slide in from end of beltStn → station pad center-left
    const stn = layout.station;
    const enter = layout.beltStn[layout.beltStn.length - 1];
    const padX = stn.x + 36;
    const padY = stn.y + stn.h/2 + 4;
    if (b.cellProgress >= 1) {
      px = padX; py = padY;
    } else {
      const k = easeInOut(b.cellProgress);
      px = enter.x + (padX - enter.x) * k;
      py = enter.y + (padY - enter.y) * k;
    }
  } else {
    return;
  }

  // Stall pulse — kept as state for logic but no visual bounce.
  const yOff = 0, scale = 1;

  drawBoxAt(ctx, px, py + yOff, b, scale, palette, mode);
  if (screenMap) screenMap.set(b.id, { x: px, y: py + yOff, sz: 22 * scale, box: b });
}

function drawBoxAt(ctx, x, y, b, scale, palette, mode) {
  const sz = 22 * scale;
  const half = sz/2;
  const klass = b.klass || 'normal';
  const pp = PRIORITY_PALETTE[klass];
  // shadow
  ctx.fillStyle = 'rgba(0,0,0,0.4)';
  ctx.fillRect(x - half + 1, y + half - 1, sz, 3);
  // crate body — priority color
  ctx.fillStyle = pp.body;
  ctx.fillRect(x - half, y - half, sz, sz);
  // top stripe — priority bright
  ctx.fillStyle = pp.stripe;
  ctx.fillRect(x - half, y - half, sz, 4);
  // bottom-left tag swatch (small color square indicating tag)
  ctx.fillStyle = tagColor(b.tag);
  ctx.fillRect(x - half + 2, y + half - 5, 4, 3);
  // banding
  ctx.strokeStyle = 'rgba(0,0,0,0.32)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(x - half, y - half + sz*0.4);
  ctx.lineTo(x + half, y - half + sz*0.4);
  ctx.stroke();
  // border
  ctx.strokeStyle = klass === 'urgent' ? 'rgba(255,160,160,0.45)' : 'rgba(255,255,255,0.18)';
  ctx.strokeRect(x - half + 0.5, y - half + 0.5, sz - 1, sz - 1);
  // stamp if completed
  if (b.stamped) {
    ctx.fillStyle = palette.good;
    ctx.fillRect(x - 6, y - 4, 12, 8);
    ctx.fillStyle = '#050608';
    ctx.font = 'bold 6px ui-monospace, monospace';
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    ctx.fillText('OK', x, y);
  }
}

function drawBot(ctx, st, layout, palette) {
  const x0 = layout.beltIn[0].x - layout.cell/2;
  const x1 = layout.beltOut[layout.beltOut.length-1].x + layout.cell/2;
  const x = x0 + (x1 - x0) * st.botX;
  const y = layout.beltY - 22;
  // body
  ctx.fillStyle = '#1c1f23';
  ctx.fillRect(x - 8, y, 16, 14);
  ctx.fillStyle = '#2a2e34';
  ctx.fillRect(x - 7, y + 2, 14, 5);
  // eye
  ctx.fillStyle = palette.good;
  ctx.beginPath(); ctx.arc(x + 1.5, y + 4, 1.5, 0, Math.PI*2); ctx.fill();
  // wheels
  ctx.fillStyle = '#0d0f12';
  ctx.beginPath(); ctx.arc(x - 5, y + 14, 2, 0, Math.PI*2); ctx.fill();
  ctx.beginPath(); ctx.arc(x + 5, y + 14, 2, 0, Math.PI*2); ctx.fill();
  // brush trail
  ctx.fillStyle = 'rgba(255,255,255,0.04)';
  ctx.fillRect(x - 24, y + 14, 18, 2);
}

function drawInspectionFlag(ctx, n, st) {
  const x = n.x + n.w - 24, y = n.y - 18;
  // pole
  ctx.strokeStyle = '#5a6068'; ctx.lineWidth = 1;
  ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x, y + 22); ctx.stroke();
  // flag (waving)
  const wave = Math.sin(st.time * 4);
  ctx.fillStyle = '#f4b740';
  ctx.beginPath();
  ctx.moveTo(x, y);
  ctx.lineTo(x + 14 + wave*1.5, y + 4);
  ctx.lineTo(x + 14 + wave*1.5, y + 10);
  ctx.lineTo(x, y + 10);
  ctx.closePath();
  ctx.fill();
  // label
  ctx.fillStyle = '#f4b740';
  ctx.font = '8px ui-monospace, monospace';
  ctx.textAlign = 'left';
  ctx.fillText('INSPECTION', x + 16, y + 8);
}

function drawParked(ctx, layout, st, counts, palette, mode) {
  const { parkedBox, parked, parkedSlots } = layout;
  const n = Math.min(parkedSlots, counts && counts.rollover ? counts.rollover : 0);
  // shelf bracket
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(parkedBox.x, parkedBox.y, parkedBox.w, parkedBox.h);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(parkedBox.x + 0.5, parkedBox.y + 0.5, parkedBox.w - 1, parkedBox.h - 1);
  // top label
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace';
  ctx.textAlign = 'left';
  ctx.fillText('PARKED · ROLLS NEXT WK', parkedBox.x + 4, parkedBox.y - 5);
  // count
  if (counts && counts.rollover > 0) {
    ctx.fillStyle = '#f4b740';
    ctx.textAlign = 'right';
    ctx.fillText(`${counts.rollover}`, parkedBox.x + parkedBox.w - 4, parkedBox.y - 5);
  }
  // slot ticks (rail)
  ctx.strokeStyle = '#15181b';
  for (let i = 0; i < parkedSlots; i++) {
    const sx = parked[i].x;
    ctx.beginPath();
    ctx.moveTo(sx - 14, parkedBox.y + parkedBox.h - 6);
    ctx.lineTo(sx + 14, parkedBox.y + parkedBox.h - 6);
    ctx.stroke();
  }
  // boxes (warn-tinted, dimmer than active)
  for (let i = 0; i < n; i++) {
    const p = parked[i];
    const sz = 18;
    ctx.fillStyle = '#5a3f10';
    ctx.fillRect(p.x - sz/2, p.y - sz/2, sz, sz);
    ctx.fillStyle = '#f4b740';
    ctx.fillRect(p.x - sz/2, p.y - sz/2, sz, 3);
    ctx.strokeStyle = 'rgba(255,255,255,0.12)';
    ctx.strokeRect(p.x - sz/2 + 0.5, p.y - sz/2 + 0.5, sz - 1, sz - 1);
    // strap detail
    ctx.strokeStyle = 'rgba(0,0,0,0.32)';
    ctx.beginPath();
    ctx.moveTo(p.x - sz/2, p.y);
    ctx.lineTo(p.x + sz/2, p.y);
    ctx.stroke();
  }
  // arrow icon (rolls forward to next week)
  if (n > 0) {
    const ax = parkedBox.x + parkedBox.w + 6, ay = parkedBox.y + parkedBox.h/2;
    ctx.strokeStyle = '#f4b740';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(ax, ay - 4);
    ctx.lineTo(ax + 6, ay);
    ctx.lineTo(ax, ay + 4);
    ctx.stroke();
  }
}

function drawRest(ctx, layout, st, counts, palette, mode) {
  const cx = layout.rest.x;
  const cy = layout.rest.y;
  const n = Math.min(8, counts && counts.stale ? counts.stale : 0);
  // bin shell
  const w = 84, h = 36;
  const x = cx - w/2, y = cy - h/2;
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(x, y, w, h);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
  // hatched bottom (rest-pile texture)
  ctx.strokeStyle = '#15181b';
  ctx.lineWidth = 1;
  for (let i = -10; i < w + 10; i += 4) {
    ctx.beginPath();
    ctx.moveTo(x + i, y + h);
    ctx.lineTo(x + i + 8, y);
    ctx.stroke();
  }
  ctx.fillStyle = '#0a0c0f';
  ctx.fillRect(x + 1, y + 1, w - 2, h - 4);
  ctx.strokeStyle = '#1c1f23';
  ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
  // dimmed crumple-boxes inside
  for (let i = 0; i < n; i++) {
    const bx = x + 8 + (i % 4) * 17;
    const by = y + 8 + Math.floor(i / 4) * 13;
    const sz = 10 + (i % 3);
    ctx.fillStyle = '#1c1f23';
    ctx.fillRect(bx, by, sz, sz);
    ctx.strokeStyle = 'rgba(255,255,255,0.08)';
    ctx.strokeRect(bx + 0.5, by + 0.5, sz - 1, sz - 1);
  }
  // labels
  ctx.fillStyle = '#5a6068';
  ctx.font = '8px ui-monospace, monospace';
  ctx.textAlign = 'left';
  ctx.fillText('REST · NO SHAME', x + w + 6, y + 9);
  if (counts && counts.stale > 0) {
    ctx.fillStyle = '#5a6068';
    ctx.font = '9px ui-monospace, monospace';
    ctx.fillText(`${counts.stale} stale`, x + w + 6, y + 22);
  }
  // chute hint from intake to rest pile
  ctx.strokeStyle = '#15181b';
  ctx.setLineDash([2, 3]);
  ctx.beginPath();
  ctx.moveTo(cx, layout.intake.y + layout.intake.h);
  ctx.lineTo(cx, y);
  ctx.stroke();
  ctx.setLineDash([]);
}

window.SupplyChain = SupplyChain;
