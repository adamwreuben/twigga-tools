#!/usr/bin/env node
// run.js

const { spawnSync } = require('child_process');
const path = require('path');
const os = require('os');

const isWin = os.platform() === 'win32';
const binPath = path.join(__dirname, isWin ? 'twigga.exe' : 'twigga');

const args = process.argv.slice(2);

const result = spawnSync(binPath, args, {
    stdio: 'inherit'
});

process.exit(result.status);