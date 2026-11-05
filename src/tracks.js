/* Musical data. Everything here is synthesized at play time by engine.js. */

window.TRACKS = [
  {
    id: "dawn",
    title: "Tape Hiss at Dawn",
    subtitle: "Rhodes chords, brushed drums, and a room that never gets quite quiet.",
    bpm: 74,
    bars: 32,
    swing: 0.34,
    root: 50,                 // D3
    scale: [0, 2, 4, 5, 7, 9, 11],
    seed: 11,
    chords: [
      { root: 0,  notes: [0, 4, 7, 11, 14] },  // Dmaj9
      { root: 9,  notes: [0, 3, 7, 10, 14] },  // Bm9
      { root: 5,  notes: [0, 4, 7, 11, 14] },  // Gmaj9
      { root: 7,  notes: [0, 5, 7, 10] }       // A7sus
    ],
    drums: {
      kick:  "x.....x..x..x...",
      snare: "....x.......x...",
      hat:   "x.x.x.x.x.x.x.x.",
      rim:   "..............x."
    },
    melody: "x...x..x..x..x..",
    bass: "x.......x......."
  },
  {
    id: "rain",
    title: "Rain on the Fire Escape",
    subtitle: "A minor, wide reverb, and a snare that arrives a little late on purpose.",
    bpm: 68,
    bars: 28,
    swing: 0.42,
    root: 57,                 // A3
    scale: [0, 3, 5, 7, 10],
    seed: 29,
    chords: [
      { root: 0,  notes: [0, 3, 7, 10, 14] },  // Am9
      { root: 8,  notes: [0, 4, 7, 11] },      // Fmaj7
      { root: 3,  notes: [0, 4, 7, 11] },      // Cmaj7
      { root: 10, notes: [0, 4, 7, 9] }        // G6
    ],
    drums: {
      kick:  "x......x..x.....",
      snare: "....x.......x...",
      hat:   "..x...x...x...x.",
      rim:   "........x......."
    },
    melody: "x..x...x..x..x..",
    bass: "x.......x......."
  },
  {
    id: "elevator",
    title: "Slow Elevator",
    subtitle: "E flat major, soft brushes, the feeling of going up one floor too far.",
    bpm: 82,
    bars: 36,
    swing: 0.28,
    root: 51,                 // Eb3
    scale: [0, 2, 4, 5, 7, 9, 11],
    seed: 47,
    chords: [
      { root: 0,  notes: [0, 4, 7, 11, 14] },  // Ebmaj9
      { root: 9,  notes: [0, 3, 7, 10, 14] },  // Cm9
      { root: 5,  notes: [0, 4, 7, 11] },      // Abmaj7
      { root: 7,  notes: [0, 4, 7, 10] }       // Bb7
    ],
    drums: {
      kick:  "x.....x.....x...",
      snare: "....x.......x...",
      hat:   "x.xxx.x.x.xxx.x.",
      rim:   "....x.......x..."
    },
    melody: "x...x.x.x...x...",
    bass: "x.....x...x....."
  },
  {
    id: "blue",
    title: "Blue Hour Commute",
    subtitle: "C major over a slow train rhythm. Headphones, window, nothing else.",
    bpm: 70,
    bars: 30,
    swing: 0.36,
    root: 48,                 // C3
    scale: [0, 2, 4, 5, 7, 9, 11],
    seed: 73,
    chords: [
      { root: 0,  notes: [0, 4, 7, 11, 14] },  // Cmaj9
      { root: 9,  notes: [0, 3, 7, 10, 14] },  // Am9
      { root: 5,  notes: [0, 4, 7, 11] },      // Fmaj7
      { root: 7,  notes: [0, 4, 7, 10] }       // G7
    ],
    drums: {
      kick:  "x..x....x..x....",
      snare: "....x.......x...",
      hat:   "x.x.x.x.x.x.x.x.",
      rim:   "..............x."
    },
    melody: "x...x..x..x..xx.",
    bass: "x.......x...x..."
  },
  {
    id: "warm",
    title: "Cassette Warm",
    subtitle: "G major, detuned keys, and a hiss that behaves like an instrument.",
    bpm: 78,
    bars: 30,
    swing: 0.3,
    root: 55,                 // G3
    scale: [0, 2, 4, 5, 7, 9, 11],
    seed: 97,
    chords: [
      { root: 0,  notes: [0, 4, 7, 11, 14] },  // Gmaj7 (add9)
      { root: 9,  notes: [0, 3, 7, 10, 14] },  // Em9
      { root: 5,  notes: [0, 4, 7, 11, 14] },  // Cmaj9
      { root: 7,  notes: [0, 4, 7, 9, 14] }    // D6/9
    ],
    drums: {
      kick:  "x.....x...x..x..",
      snare: "....x.......x...",
      hat:   "x..x..x.x..x..x.",
      rim:   "..............x."
    },
    melody: "x...x..x.x..x...",
    bass: "x.......x......."
  }
];
