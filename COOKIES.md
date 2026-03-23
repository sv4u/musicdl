# How To Set Up YouTube Cookies for musicdl

This guide explains how to export YouTube authentication cookies so that musicdl can download age-restricted, explicit, or login-gated content. This is especially important when running musicdl in Docker (e.g. on TrueNAS Scale), where browser-based cookie extraction is not available.

## Prerequisites

- A YouTube/Google account logged in to one of the supported browsers
- musicdl v1.3+ with the `cookies` config option
- For Docker deployments: access to mount a file into the container

## Why Are Cookies Needed?

Some YouTube content requires authentication:

- **Age-restricted videos** – YouTube requires sign-in to confirm your age
- **Explicit content** – Tracks marked as explicit may be blocked without authentication
- **Region-locked content** – Some content is only available when signed in

Without cookies, yt-dlp will fail with errors like:

```
Sign in to confirm your age. This video may be inappropriate for some users.
```

## Two Approaches

### Approach A: Browser Cookie Extraction (Local Only)

If you run musicdl **directly on a machine with a browser**, use the `cookies_from_browser` config option. This tells yt-dlp to read cookies directly from your browser's cookie store.

Add to your `config.yaml` under `download`:

```yaml
download:
  cookies_from_browser: "vivaldi"  # or "chrome", "firefox", "safari", etc.
```

Supported values: `chrome`, `chromium`, `firefox`, `safari`, `vivaldi`, `brave`, `edge`, `opera`

This approach does **not** work inside Docker containers because there is no browser installed.

### Approach B: Cookies File (Docker / Remote / Headless)

Export your cookies to a Netscape-format `cookies.txt` file, then point musicdl at it. This is the recommended approach for Docker, TrueNAS, and any headless deployment.

Add to your `config.yaml` under `download`:

```yaml
download:
  cookies: "/download/cookies.txt"
```

The rest of this guide covers how to export `cookies.txt` from each supported browser.

## Exporting Cookies by Browser

### Vivaldi (Windows, macOS, Linux)

Vivaldi is Chromium-based, so it supports Chrome-compatible extensions.

#### Step 1: Install the Cookie Export Extension

1. Open Vivaldi and navigate to the Chrome Web Store:
   `https://chromewebstore.google.com`
2. Search for **"Get cookies.txt LOCALLY"** (by Rahul Shaw)
3. Click **Add to Vivaldi** (Vivaldi will show "Add to Chrome" — this is normal)
4. Confirm the installation when prompted

#### Step 2: Log In to YouTube

1. Go to `https://www.youtube.com`
2. Click **Sign in** in the top right
3. Log in with the Google account you want to use for downloads
4. Verify you are signed in (your profile picture should appear in the top right)

#### Step 3: Export the Cookies

1. While on `youtube.com`, click the **Get cookies.txt LOCALLY** extension icon in the toolbar
2. If you don't see the icon, click the puzzle piece icon (Extensions) in the toolbar, then click the extension
3. Click **Export** or **Get cookies.txt** (the exact label may vary by version)
4. Choose **Current Site** to export only YouTube cookies (recommended for security)
5. Save the file as `cookies.txt`

#### Step 4: Place the Cookies File

- **Docker / TrueNAS**: Copy `cookies.txt` to your mounted music directory (e.g. `/mnt/peace-house-storage-pool/peace-house-storage/Music/cookies.txt`), which maps to `/download/cookies.txt` inside the container
- **Local**: Place it alongside your `config.yaml` or use an absolute path in the config

### Chrome (Windows, macOS, Linux)

#### Step 1: Install the Cookie Export Extension

1. Open Chrome and go to the Chrome Web Store:
   `https://chromewebstore.google.com`
2. Search for **"Get cookies.txt LOCALLY"** (by Rahul Shaw)
3. Click **Add to Chrome**
4. Confirm by clicking **Add extension**

#### Step 2: Log In to YouTube

1. Go to `https://www.youtube.com`
2. Click **Sign in** in the top right
3. Log in with your Google account
4. Verify you are signed in (your profile picture should appear in the top right)

#### Step 3: Export the Cookies

1. Navigate to `https://www.youtube.com` (stay on this domain)
2. Click the **Get cookies.txt LOCALLY** extension icon in the toolbar
3. If the icon is hidden, click the puzzle piece icon (Extensions) in Chrome's toolbar, then click the extension
4. Click **Export** to download cookies for the current site
5. Save the file as `cookies.txt`

#### Step 4: Place the Cookies File

- **Docker / TrueNAS**: Copy `cookies.txt` to your mounted music directory
- **Local**: Place it where your config can reference it, or use `cookies_from_browser: "chrome"` instead (no file needed for local Chrome)

### Firefox (Windows, macOS, Linux)

#### Step 1: Install the Cookie Export Add-on

1. Open Firefox and go to Firefox Add-ons:
   `https://addons.mozilla.org`
2. Search for **"cookies.txt"** (by Lennon Hill) or **"Get cookies.txt LOCALLY"**
3. Click **Add to Firefox**
4. Confirm the installation when prompted
5. Optionally, allow the add-on in private windows if you browse YouTube in private mode

#### Step 2: Log In to YouTube

1. Go to `https://www.youtube.com`
2. Click **Sign in** in the top right
3. Log in with your Google account
4. Verify you are signed in

#### Step 3: Export the Cookies

1. Navigate to `https://www.youtube.com`
2. Click the **cookies.txt** add-on icon in the toolbar
3. If the icon is not visible, click the hamburger menu (three lines) → **Add-ons and themes** → find the extension and click its icon
4. Select **Current Site** to export only YouTube cookies
5. Click **Export** or **Download**
6. Save the file as `cookies.txt`

#### Step 4: Place the Cookies File

- **Docker / TrueNAS**: Copy `cookies.txt` to your mounted music directory
- **Local**: Place it where your config can reference it, or use `cookies_from_browser: "firefox"` instead

### Safari (macOS)

Safari does not support the same extension ecosystem as Chrome or Firefox. There are two methods to export cookies from Safari.

#### Method 1: Using a Companion Browser (Recommended)

The simplest approach is to:

1. Open **Chrome** or **Firefox** on your Mac
2. Go to `https://www.youtube.com` and sign in with the same Google account
3. Follow the Chrome or Firefox instructions above to export `cookies.txt`

#### Method 2: Using the yt-dlp Built-in Extractor (Local macOS Only)

If you run musicdl directly on your Mac (not in Docker), yt-dlp can read Safari cookies natively:

```yaml
download:
  cookies_from_browser: "safari"
```

This uses macOS Keychain access to decrypt Safari's cookie store. You may see a system prompt asking you to allow access — click **Allow** or **Always Allow**.

This method only works locally on macOS and cannot be used for Docker deployments.

#### Method 3: Using a JavaScript Bookmarklet

1. Go to `https://www.youtube.com` and sign in
2. Open Safari's **Web Inspector** (Develop → Show Web Inspector, or Option+Command+I)
3. Go to the **Console** tab
4. Paste and run the following script:

```javascript
(function() {
    var cookies = document.cookie.split('; ');
    var output = '# Netscape HTTP Cookie File\n';
    cookies.forEach(function(c) {
        var parts = c.split('=');
        var name = parts[0];
        var value = parts.slice(1).join('=');
        output += '.youtube.com\tTRUE\t/\tTRUE\t0\t' + name + '\t' + value + '\n';
    });
    var blob = new Blob([output], {type: 'text/plain'});
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'cookies.txt';
    a.click();
})();
```

5. Save the downloaded `cookies.txt` file

**Note:** The bookmarklet approach captures only first-party cookies visible to JavaScript. Some HTTP-only cookies (like authentication tokens) may not be included, which can cause yt-dlp authentication to fail. If this happens, use Method 1 (companion browser) instead.

## Configuring musicdl

After exporting `cookies.txt`, update your `config.yaml`:

### For Docker / TrueNAS Deployments

```yaml
download:
  cookies: "/download/cookies.txt"
  js_runtimes: "node"
```

Place `cookies.txt` in the same directory that is mounted as `/download` in the container.

For TrueNAS Scale with the default compose configuration, this means placing it at:

```
/mnt/peace-house-storage-pool/peace-house-storage/Music/cookies.txt
```

### For Local Deployments

```yaml
download:
  cookies_from_browser: "vivaldi"  # reads cookies directly from browser
```

Or, if you prefer the file approach locally:

```yaml
download:
  cookies: "/path/to/cookies.txt"
```

### JavaScript Runtime (Docker)

Modern versions of yt-dlp require a JavaScript runtime for YouTube extraction. The Docker image includes Node.js, so add this to your config:

```yaml
download:
  js_runtimes: "node"
```

This is already included in the Docker image and only needs to be enabled in the config. For local installations, yt-dlp will auto-detect available runtimes (Node.js, Deno, or Bun).

## Verifying the Setup

### Test Locally

```bash
# Test that yt-dlp can use your cookies
yt-dlp --cookies cookies.txt --js-runtimes node -v "https://www.youtube.com/watch?v=EXAMPLE"
```

### Test in Docker

```bash
# Shell into the running container
docker exec -it <container-name> sh

# Test yt-dlp with cookies
yt-dlp --cookies /download/cookies.txt --js-runtimes node -v "https://www.youtube.com/watch?v=EXAMPLE"
```

## Troubleshooting

- **"Sign in to confirm your age"** — The cookies file is missing, expired, or does not contain valid YouTube authentication. Re-export from your browser while logged in.

- **"No supported JavaScript runtime could be found"** — Add `js_runtimes: "node"` to the `download` section of your config. The Docker image includes Node.js; local installs need Node.js, Deno, or Bun on PATH.

- **Cookies expire after a few weeks** — YouTube/Google session cookies have a limited lifetime. If downloads start failing with authentication errors, re-export `cookies.txt` from your browser.

- **"Could not find a suitable TLS library"** — This is a yt-dlp issue unrelated to cookies. Ensure yt-dlp is up to date: `pip install --upgrade yt-dlp`.

- **Extension not visible in toolbar** — In Vivaldi/Chrome, click the puzzle piece icon. In Firefox, check the overflow menu (>>). Pin the extension to the toolbar for easier access.

- **Multiple Google accounts** — Make sure you are logged into YouTube with the correct account before exporting. If you have multiple accounts, check the profile picture matches the account you want to use.

## Security Notes

- `cookies.txt` contains your YouTube session credentials. Treat it like a password.
- Do not commit `cookies.txt` to version control. It is included in `.gitignore` by default.
- The **"Get cookies.txt LOCALLY"** extension processes cookies entirely in your browser without sending data to external servers. Avoid extensions that upload cookies to remote services.
- If you suspect your cookies have been compromised, sign out of all Google sessions at [myaccount.google.com/security](https://myaccount.google.com/security).

## Additional Information

- The `cookies` and `cookies_from_browser` options are mutually exclusive. If both are set, `cookies` (file path) takes precedence.
- Supported `js_runtimes` values: `node`, `deno`, `bun`, `quickjs` (whatever is installed on the system).
- For the full configuration reference, see [README.md](README.md#configuration).
- For MCP server setup (AI-assisted visibility), see [MCP.md](MCP.md).
