#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import process from 'node:process';

const base = process.argv[2];
const head = process.argv[3] ?? 'HEAD';
if ((base && !process.argv[3]) || process.argv.length > 4) {
  process.stderr.write(
    'Usage: check-changelog-tags.mjs [<base-ref> <head-ref>]\n'
  );
  process.exit(2);
}

const tags = execFileSync(
  'git',
  ['tag', '--merged', head, '--list', 'v[0-9]*', '--sort=version:refname'],
  { encoding: 'utf8' }
)
  .trim()
  .split('\n')
  .filter((tag) => /^v\d+\.\d+\.\d+$/.test(tag));

function releaseSection(changelog, tag) {
  const lines = changelog.replace(/\r\n?/gu, '\n').split('\n');
  const start = lines.findIndex(
    (line) =>
      line === `## [${tag}]` ||
      line.startsWith(`## [${tag}] `) ||
      line.startsWith(`## ${tag} `)
  );
  if (start < 0) {
    return '';
  }
  let end = lines.findIndex((line, index) => index > start && /^## /.test(line));
  if (end < 0) {
    end = lines.length;
  }
  return lines.slice(start, end).join('\n').trim();
}

const changelog = execFileSync(
  'git',
  ['show', `${head}:docs/CHANGELOG.md`],
  { encoding: 'utf8' }
);
const headings = [...changelog.matchAll(/^## \[?(v\d+\.\d+\.\d+)\]?/gmu)].map(
  (match) => match[1]
);
const counts = new Map();
for (const tag of headings) {
  counts.set(tag, (counts.get(tag) ?? 0) + 1);
}

const missing = tags.filter((tag) => !counts.has(tag));
const duplicate = tags.filter((tag) => (counts.get(tag) ?? 0) > 1);
if (missing.length > 0 || duplicate.length > 0) {
  if (missing.length > 0) {
    process.stderr.write(`Changelog is missing tagged releases: ${missing.join(', ')}\n`);
  }
  if (duplicate.length > 0) {
    process.stderr.write(`Changelog has duplicate release sections: ${duplicate.join(', ')}\n`);
  }
  process.exit(1);
}

if (base) {
  const baseChangelog = execFileSync('git', ['show', `${base}:docs/CHANGELOG.md`], {
    encoding: 'utf8',
  });
  const headChangelog = execFileSync('git', ['show', `${head}:docs/CHANGELOG.md`], {
    encoding: 'utf8',
  });
  const baseTags = execFileSync(
    'git',
    ['tag', '--merged', base, '--list', 'v[0-9]*'],
    { encoding: 'utf8' }
  )
    .trim()
    .split('\n')
    .filter((tag) => /^v\d+\.\d+\.\d+$/.test(tag));
  const modified = baseTags.filter((tag) => {
    const before = releaseSection(baseChangelog, tag);
    return before && releaseSection(headChangelog, tag) !== before;
  });
  if (modified.length > 0) {
    process.stderr.write(
      `Published changelog sections are append-only; these sections changed: ${modified.join(', ')}\n`
    );
    process.exit(1);
  }
}

process.stdout.write(`Changelog covers all ${tags.length} tagged release(s).\n`);
