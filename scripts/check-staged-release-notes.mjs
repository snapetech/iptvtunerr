#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import process from 'node:process';

import {
  isReleaseNoteFile,
  isReleaseNoteShipped,
  parseReleaseNote,
} from './release-notes.mjs';

const changes = execFileSync(
  'git',
  ['diff', '--cached', '--name-status', '--find-renames', '--', 'release-notes'],
  { encoding: 'utf8' }
)
  .split('\n')
  .filter(Boolean)
  .map((line) => {
    const [status, source, destination] = line.split('\t');
    return {
      status: status.charAt(0),
      file: status.startsWith('R') ? destination : source,
      previousFile: status.startsWith('R') ? source : undefined,
    };
  })
  .filter(({ file }) => file && isReleaseNoteFile(file));

const errors = [];
for (const change of changes) {
  if (change.status === 'D') {
    if (isReleaseNoteShipped(change.file, 'HEAD')) {
      errors.push(`${change.file}: shipped fragments cannot be deleted`);
    }
    continue;
  }

  if (
    change.status !== 'A' &&
    [change.file, change.previousFile]
      .filter(Boolean)
      .some((file) => isReleaseNoteShipped(file, 'HEAD'))
  ) {
    errors.push(`${change.file}: shipped fragments are append-only`);
    continue;
  }

  let content;
  try {
    content = execFileSync('git', ['show', `:${change.file}`], {
      encoding: 'utf8',
    });
  } catch (error) {
    errors.push(`${change.file}: unable to read staged content (${error.message})`);
    continue;
  }

  const note = parseReleaseNote(change.file, content);
  errors.push(...note.errors.map((error) => `${change.file}: ${error}`));
}

if (errors.length > 0) {
  process.stderr.write('Staged release-note validation failed:\n');
  for (const error of errors) {
    process.stderr.write(`- ${error}\n`);
  }
  process.exit(1);
}

process.stdout.write(
  changes.length > 0
    ? `Validated ${changes.length} staged release-note fragment(s).\n`
    : 'No staged release-note fragments to validate.\n'
);
