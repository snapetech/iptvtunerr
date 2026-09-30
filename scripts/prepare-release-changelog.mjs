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
const version = args.get('--version');
const head = args.get('--head') ?? 'HEAD';
const requestedDate = args.get('--date');

if (!version || !/^v\d+\.\d+\.\d+$/.test(version)) {
  process.stderr.write(
    'Usage: prepare-release-changelog.mjs --version vX.Y.Z [--head <ref>] [--date YYYY-MM-DD]\n'
  );
  process.exit(2);
}

const workingChanges = execFileSync(
  'git',
  ['status', '--porcelain', '--untracked-files=all'],
  { encoding: 'utf8' }
).trim();
if (workingChanges) {
  process.stderr.write(
    'The release worktree must be clean before preparing the changelog. Commit release-note fragments first.\n'
  );
  process.exit(1);
}

try {
  execFileSync('git', ['show-ref', '--verify', '--quiet', `refs/tags/${version}`], {
    stdio: 'ignore',
  });
  process.stderr.write(`Release tag ${version} already exists; refusing to reuse it.\n`);
  process.exit(1);
} catch (error) {
  if (error.status !== 1) {
    throw error;
  }
}

const previousTag = latestReleaseTag(head);
if (!previousTag) {
  process.stderr.write(`No published release tag is reachable from ${head}.\n`);
  process.exit(1);
}

const existingChangelog = fs.readFileSync('docs/CHANGELOG.md', 'utf8');
if (
  extractReleaseChangelogSection(existingChangelog, version) ||
  existingChangelog.split('\n').some((line) => line.startsWith(`## [${version}]`))
) {
  process.stderr.write(`docs/CHANGELOG.md already has a section for ${version}.\n`);
  process.exit(1);
}

const entries = changedReleaseNoteFiles(previousTag, head);
const modifiedShipped = entries.filter(
  (entry) => entry.status !== 'A' && isReleaseNoteChangeShipped(entry, head)
);
const currentUnshipped = entries
  .filter(
    (entry) =>
      entry.status !== 'D' &&
      entry.status !== 'A' &&
      !isReleaseNoteChangeShipped(entry, head)
  )
  .map((entry) => ({ ...entry, status: 'A' }));
const { notes, errors } = readReleaseNotes([
  ...entries.filter((entry) => entry.status === 'A'),
  ...currentUnshipped,
]);
if (modifiedShipped.length > 0 || errors.length > 0) {
  process.stderr.write('Release-note fragments are not ready for release:\n');
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

const date = requestedDate ?? execFileSync('git', ['log', '-1', '--format=%cs', head], {
  encoding: 'utf8',
}).trim();
if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) {
  process.stderr.write(`Invalid release date: ${date}\n`);
  process.exit(2);
}

const section = `## [${version}] - ${date}\n\n${formatChangelogNotes(notes)}\n`;
const firstSection = existingChangelog.search(/^## \[/mu);
if (firstSection < 0) {
  process.stderr.write('docs/CHANGELOG.md has no existing versioned release section.\n');
  process.exit(1);
}

const updatedChangelog = `${existingChangelog.slice(0, firstSection)}${section}\n${existingChangelog.slice(firstSection)}`;
fs.writeFileSync('docs/CHANGELOG.md', updatedChangelog);
process.stdout.write(
  `Prepared ${version} in docs/CHANGELOG.md from ${notes.length} fragment(s) since ${previousTag}.\n`
);
