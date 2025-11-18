#!/usr/bin/env node

const { spawn } = require('child_process');
const mockServer = require('./mock-server.cjs');

console.log('Starting mock server...');

// Wait for server to start
setTimeout(() => {
  console.log('Mock server ready. Running BDD tests...\n');

  // Run cucumber tests
  const cucumber = spawn('npx', ['cucumber-js'], {
    stdio: 'inherit',
    cwd: process.cwd(),
  });

  cucumber.on('exit', (code) => {
    console.log('\nTests completed. Shutting down mock server...');
    mockServer.close(() => {
      process.exit(code);
    });
  });
}, 1000);
