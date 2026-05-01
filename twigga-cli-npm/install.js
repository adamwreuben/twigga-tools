const fs = require('fs');
const os = require('os');
const path = require('path');
const https = require('https');

const platform = os.platform(); // 'darwin', 'linux', 'win32'
const arch = os.arch();         // 'x64', 'arm64'

let binaryName = 'twigga-linux-amd64';
if (platform === 'darwin' && arch === 'arm64') binaryName = 'twigga-mac-arm64';
if (platform === 'darwin' && arch === 'x64') binaryName = 'twigga-mac-amd64';
if (platform === 'win32') binaryName = 'twigga-win-amd64.exe';

const version = 'v1.0.0';
const downloadUrl = `https://github.com/adamwreuben/twigga-tools/releases/download/${version}/${binaryName}`;

const binPath = path.join(__dirname, platform === 'win32' ? 'twigga.exe' : 'twigga');

console.log(`Downloading Twigga CLI for ${platform} (${arch})...`);

function download(url, destination) {
    https.get(url, (res) => {
        // Redirect for GitHub: 302)
        if ([301, 302, 307, 308].includes(res.statusCode) && res.headers.location) {
            console.log('Following redirect...');
            download(res.headers.location, destination);
            return;
        }

        if (res.statusCode !== 200) {
            console.error(`Failed to download CLI: HTTP ${res.statusCode}`);
            process.exit(1);
        }
        
        const file = fs.createWriteStream(destination);
        res.pipe(file);
        
        file.on('finish', () => {
            file.close();
            // Make the downloaded Go binary executable
            if (platform !== 'win32') {
                fs.chmodSync(destination, 0o755); 
            }
            console.log('Twigga CLI installed successfully!');
        });
    }).on('error', (err) => {
        console.error('Download error:', err.message);
        fs.unlink(destination, () => process.exit(1));
    });
}

download(downloadUrl, binPath);