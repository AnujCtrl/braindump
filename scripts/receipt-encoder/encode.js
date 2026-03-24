#!/usr/bin/env node
//
// Receipt command protocol encoder.
// Reads line-based commands from stdin, encodes ESC/POS bytes, writes to stdout.
//
// Commands: CENTER, LEFT, BOLD ON, BOLD OFF, TEXT <content>, FEED <n>
//
const EscPosEncoder = require('esc-pos-encoder');

let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => { input += chunk; });
process.stdin.on('end', () => {
  const encoder = new EscPosEncoder({ width: 32 });
  encoder.initialize();

  const lines = input.split('\n');
  for (const line of lines) {
    if (line === 'CENTER') {
      encoder.align('center');
    } else if (line === 'LEFT') {
      encoder.align('left');
    } else if (line === 'BOLD ON') {
      encoder.bold(true);
    } else if (line === 'BOLD OFF') {
      encoder.bold(false);
    } else if (line.startsWith('TEXT ')) {
      encoder.text(line.slice(5));
      encoder.newline();
    } else if (line === 'TEXT') {
      encoder.newline();
    } else if (line.startsWith('FEED ')) {
      const n = parseInt(line.slice(5), 10) || 1;
      for (let i = 0; i < n; i++) {
        encoder.newline();
      }
    }
    // Ignore unknown/empty lines
  }

  const result = encoder.encode();
  process.stdout.write(Buffer.from(result));
});
