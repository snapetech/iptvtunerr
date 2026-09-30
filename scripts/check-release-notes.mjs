#!/usr/bin/env node

import fs from 'node:fs';
import process from 'node:process';

import {
  changedReleaseNoteFiles,
  formatCaptureMetadata,
  formatCuratedNotes,
  isReleaseNoteChangeShipped,
  hasExplicitNoReleaseNote,
  readReleaseNotes,
} from './release-notes.mjs';

function parseArgs(argv) {
  const args = new Map();
  for (let index = 0; index < argv.length; index += 1) {
    const key = argv[index];
    if (!key.startsWith('--')) {
      continue;
    }
    const value = argv[index + 1];
    if (!value || value.startsWith('--')) {
      args.set(key, true);
    } else {
      args.set(key, value);
      index += 1;
    }
  }
  return args;
}

const args = parseArgs(process.argv.slice(2));
const base = args.get('--base');
const head = args.get('--head');
const bodyFile = args.get('--pr-body');
const summaryFile = args.get('--summary-file');

if (!base || !head || !bodyFile) {
  process.stderr.write(
    'Usage: check-release-notes.mjs --base <sha> --head <sha> --pr-body <file> [--summary-file <file>] [--allow-mixed-push-batch]\n'
  );
  process.exit(2);
}

const entries = changedReleaseNoteFiles(base, head);
const modifiedShipped = entries.filter(
  (entry) => entry.status !== 'A' && isReleaseNoteChangeShipped(entry, head)
);
const currentUnshipped = entries
  .filter(
    (entry) =>
      ['M', 'R'].includes(entry.status) &&
      !isReleaseNoteChangeShipped(entry, head)
  )
  .map((entry) => ({ ...entry, status: 'A' }));
const noteEntries = [
  ...entries.filter((entry) => entry.status === 'A'),
  ...currentUnshipped,
];
const { notes, errors } = readReleaseNotes(noteEntries);
const body = fs.readFileSync(bodyFile, 'utf8');
const explicitNoReleaseNote = hasExplicitNoReleaseNote(body);
const allowMixedPushBatch = args.has('--allow-mixed-push-batch');
const issues = [...errors];

for (const entry of modifiedShipped) {
  issues.push(
    `${entry.file}: shipped release-note fragments are append-only; add a new fragment instead of modifying an old one`
  );
}

if (notes.length === 0 && !explicitNoReleaseNote) {
  issues.push(
    'add a validated file under release-notes/ or explicitly mark the pull request `release-note: none` for internal-only work'
  );
}

if (notes.length > 0 && explicitNoReleaseNote && !allowMixedPushBatch) {
  issues.push(
    'choose either a release-note fragment or `release-note: none`; do not select both'
  );
}

if (issues.length > 0) {
  process.stderr.write('Release-note validation failed:\n');
  for (const issue of issues) {
    process.stderr.write(`- ${issue}\n`);
  }
  process.exit(1);
}

if (summaryFile) {
  const summary =
    notes.length > 0
      ? [
          '## Release-note preview',
          '',
          formatCuratedNotes(notes),
          '',
          formatCaptureMetadata(notes),
        ].join('\n')
      : '## Release-note preview\n\nInternal-only change; no release note will be published.';
  fs.appendFileSync(summaryFile, `${summary}\n`);
}

process.stdout.write(
  notes.length > 0
    ? `Validated ${notes.length} user-facing release-note fragment(s).\n`
    : 'Explicitly marked as internal-only; no release note required.\n'
);
