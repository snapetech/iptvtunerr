#!/usr/bin/env node

import process from 'node:process';

import {
  changedReleaseNoteFiles,
  formatCuratedNotes,
  isReleaseNoteChangeShipped,
  readReleaseNotes,
} from './release-notes.mjs';

function parseArgs(argv) {
  const args = new Map();
  for (let index = 0; index < argv.length; index += 1) {
    const key = argv[index];
    if (!key.startsWith('--')) {
      continue;
    }
    args.set(key, argv[index + 1]);
    index += 1;
  }
  return args;
}

const args = parseArgs(process.argv.slice(2));
const base = args.get('--base');
const head = args.get('--head');

if (!base || !head) {
  process.stderr.write(
    'Usage: preview-release-notes.mjs --base <sha-or-ref> --head <sha-or-ref>\n'
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
const { notes, errors } = readReleaseNotes([
  ...entries.filter((entry) => entry.status === 'A'),
  ...currentUnshipped,
]);

if (modifiedShipped.length > 0 || errors.length > 0) {
  process.stderr.write('Release-note preview failed validation:\n');
  for (const entry of modifiedShipped) {
    process.stderr.write(
      `- ${entry.file}: shipped fragments are append-only; add a new fragment instead\n`
    );
  }
  for (const error of errors) {
    process.stderr.write(`- ${error}\n`);
  }
  process.exit(1);
}

if (notes.length === 0) {
  process.stdout.write('No new release-note fragments were found in this range.\n');
} else {
  process.stdout.write(`${formatCuratedNotes(notes)}\n`);
}
