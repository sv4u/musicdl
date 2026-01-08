# Wiki Documentation

This directory contains all documentation for the GitHub Wiki. These files can be uploaded to the GitHub Wiki for the repository.

## Files

- **Home.md**: Wiki homepage with navigation
- **Architecture.md**: Detailed architecture documentation
- **CI-CD.md**: CI/CD workflows and release process
- **TrueNAS-Deployment.md**: Complete TrueNAS Scale deployment guide
- **Development.md**: Development setup and guidelines

## Uploading to GitHub Wiki

To upload these files to the GitHub Wiki:

1. Navigate to your repository on GitHub
2. Click on the **Wiki** tab
3. Click **Create the first page** or **New Page**
4. Copy and paste the content from each markdown file
5. Use the filename (without .md extension) as the page title

Alternatively, you can clone the wiki repository and push these files:

```bash
# Clone wiki repository (replace with your repo)
git clone https://github.com/sv4u/musicdl.wiki.git

# Copy files
cp docs/wiki/*.md musicdl.wiki/

# Commit and push
cd musicdl.wiki
git add .
git commit -m "docs: add wiki documentation"
git push
```

## File Structure

The wiki pages are organized as follows:

- **Home.md**: Entry point with navigation
- **Architecture.md**: Technical architecture details
- **CI-CD.md**: Continuous integration and deployment
- **TrueNAS-Deployment.md**: Deployment guide for TrueNAS Scale
- **Development.md**: Development and contribution guidelines
