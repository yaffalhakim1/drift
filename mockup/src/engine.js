/* drift — a small lo-fi synthesizer.
   Rhodes-style FM keys, boom-bap drums, tape wobble, hiss and crackle.
   No samples, no network. Every sound is built from oscillators and noise. */

window.LofiEngine = function createEngine() {
  const LOOKAHEAD = 0.14;
  const TICK = 25;

  let ctx = null;
  let master = null;
  let mix = null;
  let chordBus = null;
  let wobble = null;
  let ambience = null;
  let noiseBuffer = null;

  let track = null;
  let bpm = 74;
  let stepDur = 0.121;
  let totalSteps = 0;
  let duration = 0;

  let schedStep = 0;
  let nextTime = 0;
  let nextCrackle = 0;
  let timer = null;
  let running = false;

  let anchorPos = 0;
  let anchorTime = 0;
  let rand = Math.random;
  let longVoices = new Set();
  let cleanups = new Set();
  let wantPlay = false;

  const midiToFreq = (m) => 440 * Math.pow(2, (m - 69) / 12);

  function mulberry32(seed) {
    let a = seed >>> 0;
    return function () {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  /* ---------- graph ---------- */

  function makeNoise(seconds) {
    const len = Math.floor(ctx.sampleRate * seconds);
    const buf = ctx.createBuffer(2, len, ctx.sampleRate);
    for (let ch = 0; ch < 2; ch++) {
      const data = buf.getChannelData(ch);
      let last = 0;
      for (let i = 0; i < len; i++) {
        const white = Math.random() * 2 - 1;
        last = (last + 0.02 * white) / 1.02;      // brown noise, softer than white
        data[i] = last * 3.2;
      }
    }
    return buf;
  }

  function makeImpulse(seconds, decay) {
    const len = Math.floor(ctx.sampleRate * seconds);
    const buf = ctx.createBuffer(2, len, ctx.sampleRate);
    for (let ch = 0; ch < 2; ch++) {
      const data = buf.getChannelData(ch);
      for (let i = 0; i < len; i++) {
        data[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / len, decay);
      }
    }
    return buf;
  }

  function saturationCurve(amount) {
    const n = 1024;
    const curve = new Float32Array(n);
    for (let i = 0; i < n; i++) {
      const x = (i * 2) / n - 1;
      curve[i] = Math.tanh(x * amount) / Math.tanh(amount);
    }
    return curve;
  }

  function ensure() {
    if (ctx) return ctx;
    const AC = window.AudioContext || window.webkitAudioContext;
    ctx = new AC();

    master = ctx.createGain();
    master.gain.value = 0.8;

    const limiter = ctx.createDynamicsCompressor();
    limiter.threshold.value = -14;
    limiter.knee.value = 12;
    limiter.ratio.value = 6;
    limiter.attack.value = 0.004;
    limiter.release.value = 0.18;

    const shaper = ctx.createWaveShaper();
    shaper.curve = saturationCurve(1.5);
    shaper.oversample = "2x";

    const tone = ctx.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 7200;
    tone.Q.value = 0.4;

    mix = ctx.createGain();
    mix.gain.value = 0.9;

    mix.connect(shaper);
    shaper.connect(tone);
    tone.connect(limiter);
    limiter.connect(master);
    master.connect(ctx.destination);

    chordBus = ctx.createGain();
    chordBus.gain.value = 0.16;

    const verb = ctx.createConvolver();
    verb.buffer = makeImpulse(2.6, 2.4);
    const verbSend = ctx.createGain();
    verbSend.gain.value = 0.42;
    chordBus.connect(verbSend);
    verbSend.connect(verb);
    verb.connect(mix);
    chordBus.connect(mix);

    noiseBuffer = makeNoise(3);

    // Tape wobble: a slow LFO shared by every oscillator, so the pitch drifts together.
    const lfo = ctx.createOscillator();
    lfo.type = "sine";
    lfo.frequency.value = 0.42;
    wobble = ctx.createGain();
    wobble.gain.value = 5.5;                      // cents
    lfo.connect(wobble);
    lfo.start();

    startHiss();
    return ctx;
  }

  function startHiss() {
    const src = ctx.createBufferSource();
    src.buffer = noiseBuffer;
    src.loop = true;

    const band = ctx.createBiquadFilter();
    band.type = "bandpass";
    band.frequency.value = 2400;
    band.Q.value = 0.5;

    const hp = ctx.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.value = 900;

    const g = ctx.createGain();
    g.gain.value = 0;

    src.connect(band);
    band.connect(hp);
    hp.connect(g);
    g.connect(mix);
    src.start();
    ambience = g;
  }

  /* ---------- voices ---------- */

  function stopLong() {
    const now = ctx.currentTime;
    longVoices.forEach((node) => {
      try { node.stop(now); } catch (_) { /* already stopped */ }
    });
    longVoices.clear();
    cleanups.forEach((fn) => fn());
    cleanups.clear();
  }

  /* Every detune connection keeps the oscillator reachable from the LFO,
     so a voice must unplug itself when it dies or the graph grows forever. */
  function wireDetune(...nodes) {
    nodes.forEach((n) => wobble.connect(n.detune));
    return () => nodes.forEach((n) => {
      try { wobble.disconnect(n.detune); } catch (_) { /* already gone */ }
    });
  }

  function rhodes(freq, at, hold, vel) {
    const out = ctx.createGain();
    // A pad-like envelope, not a percussive one: the chord holds a bed under the
    // whole bar so the arrangement never falls silent before the next chord.
    out.gain.setValueAtTime(0.0001, at);
    out.gain.linearRampToValueAtTime(vel, at + 0.018);
    out.gain.linearRampToValueAtTime(vel * 0.66, at + Math.min(0.6, hold * 0.22));
    out.gain.linearRampToValueAtTime(vel * 0.44, at + hold * 0.82);
    out.gain.linearRampToValueAtTime(0.0001, at + hold);

    const car = ctx.createOscillator();
    car.type = "sine";
    car.frequency.value = freq;

    const mod = ctx.createOscillator();
    mod.type = "sine";
    mod.frequency.value = freq * 2.01;

    const modGain = ctx.createGain();
    modGain.gain.setValueAtTime(freq * 2.2, at);
    modGain.gain.exponentialRampToValueAtTime(freq * 0.5, at + Math.min(0.7, hold * 0.3));
    modGain.gain.exponentialRampToValueAtTime(freq * 0.14, at + hold);

    mod.connect(modGain);
    modGain.connect(car.frequency);
    car.connect(out);
    out.connect(chordBus);

    const unwire = wireDetune(car, mod);

    car.start(at);
    mod.start(at);
    car.stop(at + hold + 0.04);
    mod.stop(at + hold + 0.04);
    longVoices.add(car);
    longVoices.add(mod);

    const release = () => {
      unwire();
      out.disconnect();
      longVoices.delete(car);
      longVoices.delete(mod);
      cleanups.delete(release);
    };
    cleanups.add(release);
    car.onended = release;
  }

  function playChord(chord, at, hold) {
    const root = track.root + chord.root;
    chord.notes.forEach((n, i) => {
      const vel = (i === 0 ? 0.5 : 0.4) * (0.92 + rand() * 0.16);
      rhodes(midiToFreq(root + n), at + i * 0.014, hold, vel);
    });
  }

  function kick(at, vel) {
    const osc = ctx.createOscillator();
    osc.type = "sine";
    osc.frequency.setValueAtTime(132, at);
    osc.frequency.exponentialRampToValueAtTime(46, at + 0.075);

    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, at);
    g.gain.linearRampToValueAtTime(vel, at + 0.004);
    g.gain.exponentialRampToValueAtTime(0.0001, at + 0.34);

    osc.connect(g);
    g.connect(mix);
    osc.start(at);
    osc.stop(at + 0.36);
  }

  function noiseHit(at, dur, vel, type, freq, q) {
    const src = ctx.createBufferSource();
    src.buffer = noiseBuffer;
    src.playbackRate.value = 1;

    const f = ctx.createBiquadFilter();
    f.type = type;
    f.frequency.value = freq;
    f.Q.value = q;

    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, at);
    g.gain.linearRampToValueAtTime(vel, at + 0.003);
    g.gain.exponentialRampToValueAtTime(0.0001, at + dur);

    src.connect(f);
    f.connect(g);
    g.connect(mix);
    src.start(at);
    src.stop(at + dur + 0.02);
  }

  function snare(at, vel) {
    noiseHit(at, 0.19, vel, "bandpass", 1900, 0.8);
    const body = ctx.createOscillator();
    body.type = "triangle";
    body.frequency.setValueAtTime(196, at);
    body.frequency.exponentialRampToValueAtTime(150, at + 0.09);
    const g = ctx.createGain();
    g.gain.setValueAtTime(vel * 0.7, at);
    g.gain.exponentialRampToValueAtTime(0.0001, at + 0.12);
    body.connect(g);
    g.connect(mix);
    body.start(at);
    body.stop(at + 0.14);
  }

  function hat(at, vel, open) {
    noiseHit(at, open ? 0.24 : 0.05, vel, "highpass", 7800, 0.9);
  }

  function rim(at, vel) {
    noiseHit(at, 0.06, vel * 0.8, "bandpass", 2600, 2.4);
  }

  function bassNote(midi, at, hold, vel) {
    const freq = midiToFreq(midi);
    const osc = ctx.createOscillator();
    osc.type = "sine";
    osc.frequency.value = freq;

    const sub = ctx.createOscillator();
    sub.type = "triangle";
    sub.frequency.value = freq * 0.5;

    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, at);
    g.gain.linearRampToValueAtTime(vel, at + 0.018);
    g.gain.exponentialRampToValueAtTime(0.0001, at + hold);

    const lp = ctx.createBiquadFilter();
    lp.type = "lowpass";
    lp.frequency.value = 520;

    osc.connect(lp);
    sub.connect(lp);
    lp.connect(g);
    g.connect(mix);
    osc.start(at);
    sub.start(at);
    osc.stop(at + hold + 0.03);
    sub.stop(at + hold + 0.03);
  }

  function bell(freq, at, vel) {
    const osc = ctx.createOscillator();
    osc.type = "sine";
    osc.frequency.value = freq;

    const g = ctx.createGain();
    g.gain.setValueAtTime(0.0001, at);
    g.gain.linearRampToValueAtTime(vel, at + 0.006);
    g.gain.exponentialRampToValueAtTime(vel * 0.22, at + 0.12);
    g.gain.exponentialRampToValueAtTime(0.0001, at + 0.85);

    wobble.connect(osc.detune);
    osc.connect(g);
    g.connect(chordBus);
    osc.start(at);
    osc.stop(at + 0.9);

    const release = () => {
      try { wobble.disconnect(osc.detune); } catch (_) { /* already gone */ }
      g.disconnect();
      cleanups.delete(release);
    };
    cleanups.add(release);
    osc.onended = release;
  }

  function crackle(at) {
    const src = ctx.createBufferSource();
    src.buffer = noiseBuffer;
    src.playbackRate.value = 2.5;

    const hp = ctx.createBiquadFilter();
    hp.type = "highpass";
    hp.frequency.value = 3200;

    const g = ctx.createGain();
    const vel = 0.02 + rand() * 0.05;
    g.gain.setValueAtTime(vel, at);
    g.gain.exponentialRampToValueAtTime(0.0001, at + 0.03);

    src.connect(hp);
    hp.connect(g);
    g.connect(mix);
    src.start(at, Math.random() * 2);
    src.stop(at + 0.04);
  }

  /* ---------- scheduling ---------- */

  function scheduleStep(step, at) {
    const local = step % 16;
    const bar = Math.floor(step / 16);
    const chord = track.chords[bar % track.chords.length];
    const swing = track.swing || 0.3;
    const when = local % 2 === 1 ? at + stepDur * swing * 0.55 : at;
    const human = (rand() - 0.5) * 0.008;
    const t = when + human;
    const barDur = stepDur * 16;

    if (local === 0) playChord(chord, t, barDur * 1.06);

    if (track.drums.kick[local] === "x") kick(t, 0.85 + rand() * 0.15);
    if (track.drums.snare[local] === "x") snare(t, 0.3 + rand() * 0.1);
    if (track.drums.hat[local] === "x") hat(t, local % 4 === 0 ? 0.11 : 0.075, local === 14);
    if (track.drums.rim && track.drums.rim[local] === "x") rim(t, 0.14);

    // A quiet off-beat shaker keeps the bar alive between the last hat and the next chord.
    if (local % 4 === 2) hat(t, 0.045, false);

    if (track.bass[local] === "x") {
      const root = track.root + chord.root - 24;
      bassNote(root + chord.notes[0], t, stepDur * 7, 0.5);
    }

    if (track.melody[local] === "x" && rand() > 0.22) {
      const pool = chord.notes.map((n) => track.root + chord.root + n + 12);
      const note = pool[Math.floor(rand() * pool.length)];
      bell(midiToFreq(note + (rand() > 0.75 ? 12 : 0)), t, 0.075 + rand() * 0.06);
    }
  }

  function scheduler() {
    if (!running) return;

    while (nextTime < ctx.currentTime + LOOKAHEAD) {
      if (schedStep >= totalSteps) {
        running = false;
        return;
      }
      scheduleStep(schedStep, nextTime);
      nextTime += stepDur;
      schedStep++;

      while (nextCrackle < ctx.currentTime + LOOKAHEAD) {
        crackle(nextCrackle);
        nextCrackle += 0.18 + rand() * 0.9;
      }
    }

    timer = window.setTimeout(scheduler, TICK);
  }

  /* ---------- transport ---------- */

  function load(nextTrack) {
    track = nextTrack;
    bpm = track.bpm;
    stepDur = 60 / bpm / 4;
    totalSteps = track.bars * 16;
    duration = totalSteps * stepDur;
    rand = mulberry32(track.seed);
  }

  function startAt(seconds) {
    ensure();
    stopLong();
    running = true;
    schedStep = Math.min(totalSteps, Math.floor(seconds / stepDur));
    nextTime = ctx.currentTime + 0.06;
    nextCrackle = ctx.currentTime + 0.05;
    anchorPos = seconds;
    anchorTime = ctx.currentTime;
    setAmbience(true);
    window.clearTimeout(timer);
    timer = window.setTimeout(scheduler, TICK);
  }

  function setAmbience(on) {
    if (!ambience) return;
    const target = on ? 0.016 : 0;
    ambience.gain.setTargetAtTime(target, ctx.currentTime, on ? 0.4 : 0.12);
  }

  function play(nextTrack, from = 0) {
    load(nextTrack);
    wantPlay = true;
    ensure();
    const go = () => { if (wantPlay) startAt(from); };
    if (ctx.state !== "running") ctx.resume().then(go, go);
    else go();
  }

  function pause() {
    if (!track) return;
    anchorPos = position();
    wantPlay = false;
    running = false;
    window.clearTimeout(timer);
    stopLong();
    setAmbience(false);
  }

  function resume() {
    if (!ctx || !track) return;
    wantPlay = true;
    const go = () => { if (wantPlay) startAt(anchorPos); };
    if (ctx.state !== "running") ctx.resume().then(go, go);
    else go();
  }

  /* Seeking works paused too: the next resume starts from the new point. */
  function seek(seconds) {
    const target = Math.max(0, Math.min(duration, seconds));
    anchorPos = target;
    schedStep = Math.min(totalSteps, Math.floor(target / stepDur));
    if (!ctx) return;
    stopLong();
    anchorTime = ctx.currentTime;
    nextTime = ctx.currentTime + 0.03;
    nextCrackle = ctx.currentTime + 0.05;
    if (!running) return;
    window.clearTimeout(timer);
    timer = window.setTimeout(scheduler, TICK);
  }

  function position() {
    if (!track) return 0;
    if (!running) return Math.min(duration, anchorPos);
    const elapsed = ctx.state === "running" ? ctx.currentTime - anchorTime : 0;
    return Math.max(0, Math.min(duration, anchorPos + elapsed));
  }

  function setVolume(value) {
    if (!ctx) return;
    const v = Math.max(0, Math.min(1, value));
    master.gain.setTargetAtTime(v, ctx.currentTime, 0.02);
  }

  function dispose() {
    window.clearTimeout(timer);
    if (ctx) ctx.close();
    ctx = null;
  }

  return {
    ensure,
    load,
    play,
    pause,
    resume,
    seek,
    position,
    setVolume,
    dispose,
    isReady: () => ctx !== null,
    getDuration: () => duration
  };
};
