#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import process from 'node:process';

import {
  changedReleaseNoteFiles,
  extractReleaseChangelogSection,
  formatChangelogNotes,
  isReleaseNoteChangeShipped,
  latestReleaseTag,
  readReleaseNotes,
} from './release-notes.mjs';

const tag = process.argv[2];
if (!tag || !/^v\d+\.\d+\.\d+$/.test(tag)) {
  process.stderr.write('Usage: verify-release-changelog.mjs vX.Y.Z\n');
  process.exit(2);
}

execFileSync('git', ['rev-parse', '--verify', `${tag}^{commit}`], { stdio: 'ignore' });
const previousTag = latestReleaseTag(`${tag}^`);
if (!previousTag) {
  process.stderr.write(`No preceding release tag is reachable before ${tag}.\n`);
  process.exit(1);
}

const entries = changedReleaseNoteFiles(previousTag, tag);
const modifiedShipped = entries.filter(
  (entry) => entry.status !== 'A' && isReleaseNoteChangeShipped(entry, tag)
);
const currentUnshipped = entries
  .filter(
    (entry) =>
      entry.status !== 'D' &&
      entry.status !== 'A' &&
      !isReleaseNoteChangeShipped(entry, tag)
  )
  .map((entry) => ({ ...entry, status: 'A' }));
const { notes, errors } = readReleaseNotes([
  ...entries.filter((entry) => entry.status === 'A'),
  ...currentUnshipped,
]);

if (modifiedShipped.length > 0 || errors.length > 0) {
  process.stderr.write('Release-note fragments are not valid for this tag:\n');
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

const changelog = fs.readFileSync('docs/CHANGELOG.md', 'utf8');
const actual = extractReleaseChangelogSection(changelog, tag);
const expected = formatChangelogNotes(notes);
if (!actual) {
  process.stderr.write(`docs/CHANGELOG.md has no populated section for ${tag}.\n`);
  process.exit(1);
}
if (actual !== expected) {
  process.stderr.write(
    `docs/CHANGELOG.md section for ${tag} does not match release-note fragments from ${previousTag}.\nRun scripts/prepare-release-changelog.mjs --version ${tag} before creating the release tag.\n`
  );
  process.exit(1);
}

process.stdout.write(
  `Verified ${tag} changelog section against ${notes.length} release-note fragment(s).\n`
);
