# Plan: Update Changelog Generation to be More Structured

## Executive Summary

This plan outlines the enhancement of changelog generation from the current basic GitHub CLI auto-generation to a structured, categorized, and well-formatted changelog system. The current implementation uses `gh release create --generate-notes`, which provides minimal structure. This plan implements a comprehensive changelog generation system that categorizes changes by type, groups by scope, highlights breaking changes, and produces professional, readable release notes suitable for users and stakeholders.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 3-4 days**Risk Level**: Low**Priority**: Medium (improves release documentation quality)

## Current State Analysis

### Current Implementation

#### Changelog Generation (`release-and-publish.yml`)

```yaml
# Create release with auto-generated changelog
gh release create "$NEXT_TAG" \
  --title "$RELEASE_TITLE" \
  --generate-notes
```

**Current Behavior**:

- Uses GitHub CLI's `--generate-notes` flag
- Automatically groups commits by type (feat, fix, etc.)
- Basic formatting with minimal structure
- No customization or categorization
- No breaking changes section
- No scope-based grouping
- No links to PRs or issues
- Limited control over output format

### Current Commit Structure

The project uses **Conventional Commits** format:

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test changes
- `ci:` - CI/CD changes
- `chore:` - Maintenance tasks

**Example commits**:

- `refactor(config): move Spotify credentials to environment variables`
- `docs: add development plans and TODO tracking`
- `fix(downloader): handle missing album metadata`

### Limitations of Current Approach

1. **No Customization**: Can't control grouping, ordering, or formatting
2. **No Breaking Changes**: Doesn't highlight breaking changes prominently
3. **No Scope Grouping**: Doesn't group by component/scope (e.g., config, downloader, docker)
4. **No PR/Issue Links**: Doesn't include links to related PRs or issues
5. **No Security Section**: Doesn't highlight security fixes separately
6. **No Contributors**: Doesn't credit contributors
7. **Limited Formatting**: Basic markdown, no rich formatting options
8. **No CHANGELOG.md**: No persistent changelog file in repository

## Objectives

1. **Primary**: Implement structured changelog generation with categorization
2. **Primary**: Group changes by type (feat, fix, docs, etc.) and scope (config, downloader, etc.)
3. **Primary**: Highlight breaking changes prominently
4. **Secondary**: Include PR and issue links in changelog
5. **Secondary**: Maintain a CHANGELOG.md file in the repository
6. **Tertiary**: Add security fixes section
7. **Tertiary**: Credit contributors in releases

## Technical Approach

### Phase 1: Choose Changelog Generation Tool

#### Step 1.1: Evaluate Options

**Option A: git-cliff**

- **Pros**: 
- Designed for Conventional Commits
- Highly configurable via TOML
- Supports templates and custom sections
- Can generate CHANGELOG.md file
- Rust-based, fast and reliable
- **Cons**: 
- Requires installation in CI
- Additional dependency

**Option B: Custom Script (Python/Bash)**

- **Pros**: 
- Full control over format
- No external dependencies
- Can use GitHub API directly
- **Cons**: 
- More maintenance
- Need to implement all features

**Option C: release-please**

- **Pros**: 
- Google-maintained
- Handles versioning and changelog
- Integrates with GitHub
- **Cons**: 
- More complex setup
- May conflict with existing versioning

**Recommendation**: Use **git-cliff** for structured changelog generation with custom configuration.

#### Step 1.2: Install git-cliff in Workflow

Add git-cliff installation step:

```yaml
- name: Install git-cliff
  run: |
    # Install git-cliff using cargo or pre-built binary
    curl -L https://github.com/orhun/git-cliff/releases/latest/download/git-cliff-1.4.1-x86_64-unknown-linux-gnu.tar.gz | tar -xz
    sudo mv git-cliff /usr/local/bin/
    git-cliff --version
```

Or use GitHub Action:

```yaml
- name: Install git-cliff
  uses: orhun/git-cliff-action@v2
  with:
    config: .cliff.toml
```



### Phase 2: Configure git-cliff

#### Step 2.1: Create Configuration File

Create `.cliff.toml` configuration file:

```toml
[changelog]
# Changelog header
header = """
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
"""

# Footer
footer = """
---
**Full Changelog**: https://github.com/sv4u/musicdl/compare/{previous_tag}...{new_tag}
"""

# Template for changelog body
body = """
{% if version %}\
## [{{ version }}](https://github.com/sv4u/musicdl/releases/tag/{{ version }}) - {{ timestamp | date(format="%Y-%m-%d") }}

{% else %}\
## [Unreleased]

{% endif %}\
{% for group, commits in commits | group_by(attribute="group") %}
### {{ group | upper_first }}

{% for commit in commits %}
{% if commit.scope -%}
- **{{ commit.scope }}**: {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% else -%}
- {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% endif %}
{% endfor %}

{% endfor %}
"""

# Commit parsing
[git]
conventional_commits = true
filter_unconventional = true
split_commits = false
commit_parsers = [
    { message = "^feat", group = "Features", default_scope = "general" },
    { message = "^fix", group = "Bug Fixes", default_scope = "general" },
    { message = "^docs", group = "Documentation", default_scope = "general" },
    { message = "^refactor", group = "Refactoring", default_scope = "general" },
    { message = "^perf", group = "Performance", default_scope = "general" },
    { message = "^test", group = "Tests", default_scope = "general" },
    { message = "^ci", group = "CI/CD", default_scope = "general" },
    { message = "^chore", group = "Chores", default_scope = "general" },
    { message = "^build", group = "Build", default_scope = "general" },
    { message = "^style", group = "Style", default_scope = "general" },
]

# Link parsing (for PR/issue references)
[git.link_parsers]
# Match PR references: (#123) or (PR #123)
pattern = '\(#(\d+)\)|\(PR #(\d+)\)'
href = "https://github.com/sv4u/musicdl/pull/$1"
text = "#$1"

# Match issue references: (#456) or (issue #456)
pattern = '\(#(\d+)\)|\(issue #(\d+)\)'
href = "https://github.com/sv4u/musicdl/issues/$1"
text = "#$1"

# Tag processing
[git.tag_pattern]
pattern = '^v(.*)$'
```



#### Step 2.2: Customize for Project Structure

Add scope-based grouping for project components:

```toml
[git.commit_parsers]
# Features by scope
{ message = "^feat\\(config\\)", group = "Features", scope = "Configuration" },
{ message = "^feat\\(downloader\\)", group = "Features", scope = "Downloader" },
{ message = "^feat\\(docker\\)", group = "Features", scope = "Docker" },
{ message = "^feat\\(ci\\)", group = "Features", scope = "CI/CD" },

# Fixes by scope
{ message = "^fix\\(config\\)", group = "Bug Fixes", scope = "Configuration" },
{ message = "^fix\\(downloader\\)", group = "Bug Fixes", scope = "Downloader" },
{ message = "^fix\\(docker\\)", group = "Bug Fixes", scope = "Docker" },
{ message = "^fix\\(ci\\)", group = "Bug Fixes", scope = "CI/CD" },
```



### Phase 3: Implement Breaking Changes Detection

#### Step 3.1: Configure Breaking Changes

Add breaking changes detection in git-cliff config:

```toml
[git.commit_parsers]
# Breaking changes (feat! or BREAKING CHANGE)
{ message = "^feat!", group = "Breaking Changes", breaking = true },
{ message = "BREAKING CHANGE", group = "Breaking Changes", breaking = true },
```



#### Step 3.2: Update Template for Breaking Changes

Modify template to highlight breaking changes:

```toml
body = """
{% if version %}\
## [{{ version }}](https://github.com/sv4u/musicdl/releases/tag/{{ version }}) - {{ timestamp | date(format="%Y-%m-%d") }}

{% else %}\
## [Unreleased]

{% endif %}\
{% if breaking.len() > 0 %}
### ⚠️ Breaking Changes

{% for commit in breaking %}
- **{{ commit.scope }}**: {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% endfor %}

{% endif %}\
{% for group, commits in commits | group_by(attribute="group") %}
### {{ group | upper_first }}

{% for commit in commits %}
{% if commit.scope -%}
- **{{ commit.scope }}**: {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% else -%}
- {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% endif %}
{% endfor %}

{% endfor %}
"""
```



### Phase 4: Update Release Workflow

#### Step 4.1: Generate Changelog with git-cliff

Replace GitHub CLI changelog generation:

```yaml
- name: Generate changelog
  id: changelog
  run: |
    PREVIOUS_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    NEXT_TAG="${{ steps.next-version.outputs.tag }}"
    
    if [ -z "$PREVIOUS_TAG" ]; then
      # First release - generate from all commits
      CHANGELOG=$(git-cliff --tag "$NEXT_TAG" --output -)
    else
      # Generate changelog from previous tag
      CHANGELOG=$(git-cliff --tag "$NEXT_TAG" --latest --output -)
    fi
    
    # Save changelog to file for release
    echo "$CHANGELOG" > CHANGELOG_RELEASE.md
    
    # Also update CHANGELOG.md in repository
    if [ -f CHANGELOG.md ]; then
      # Prepend new changelog to existing file
      echo "$CHANGELOG" > CHANGELOG_NEW.md
      cat CHANGELOG.md >> CHANGELOG_NEW.md
      mv CHANGELOG_NEW.md CHANGELOG.md
    else
      # Create new CHANGELOG.md
      echo "$CHANGELOG" > CHANGELOG.md
    fi
    
    # Output changelog for release
    echo "changelog<<EOF" >> $GITHUB_OUTPUT
    echo "$CHANGELOG" >> $GITHUB_OUTPUT
    echo "EOF" >> $GITHUB_OUTPUT

- name: Create GitHub release with structured changelog
  if: ${{ inputs.dry_run != 'true' }}
  id: create-release
  env:
    GITHUB_TOKEN: ${{ secrets.RELEASE_PAT || secrets.GITHUB_TOKEN }}
  run: |
    NEXT_TAG="${{ steps.next-version.outputs.tag }}"
    RELEASE_TITLE="Release $NEXT_TAG"
    CHANGELOG="${{ steps.changelog.outputs.changelog }}"
    
    # Create release with structured changelog
    if gh release create "$NEXT_TAG" \
      --title "$RELEASE_TITLE" \
      --notes "$CHANGELOG"; then
      echo "[OK] Release $NEXT_TAG created successfully"
    else
      RELEASE_EXIT=$?
      echo "[X] Error: Failed to create release (exit code: $RELEASE_EXIT)"
      exit $RELEASE_EXIT
    fi
```



#### Step 4.2: Commit CHANGELOG.md Updates

Optionally commit CHANGELOG.md back to repository:

```yaml
- name: Commit CHANGELOG.md
  if: ${{ inputs.dry_run != 'true' }}
  run: |
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    git add CHANGELOG.md
    git commit -m "docs: update CHANGELOG.md for ${{ steps.next-version.outputs.tag }}" || exit 0
    git push origin main || exit 0
```



### Phase 5: Add Security Fixes Section

#### Step 5.1: Configure Security Detection

Add security fix detection:

```toml
[git.commit_parsers]
# Security fixes
{ message = "^fix\\(security\\)", group = "Security", scope = "Security" },
{ message = "security", group = "Security", scope = "Security", skip = false },
```



#### Step 5.2: Update Template

Add security section to template:

```toml
body = """
{% if version %}\
## [{{ version }}](https://github.com/sv4u/musicdl/releases/tag/{{ version }}) - {{ timestamp | date(format="%Y-%m-%d") }}

{% else %}\
## [Unreleased]

{% endif %}\
{% if breaking.len() > 0 %}
### ⚠️ Breaking Changes
...
{% endif %}\
{% for group, commits in commits | filter(attribute="group", value="Security") %}
### 🔒 Security

{% for commit in commits %}
- **{{ commit.scope }}**: {{ commit.message | upper_first }}{% if commit.links %} ({{ commit.links | join(sep=", ") }}){% endif %}
{% endfor %}

{% endfor %}\
{% for group, commits in commits | group_by(attribute="group") | filter(attribute="group", value!="Security") %}
### {{ group | upper_first }}
...
{% endfor %}
"""
```



### Phase 6: Add Contributor Credits

#### Step 6.1: Configure Contributor Detection

git-cliff can extract contributors from commits:

```toml
[git.conventional_commits]
# Extract author information
authors = [
    { name = ".*", email = ".*" }
]
```



#### Step 6.2: Add Contributors Section

Update template to include contributors:

```toml
footer = """
---
**Contributors**: {{ authors | map(attribute="name") | join(sep=", ") }}
**Full Changelog**: https://github.com/sv4u/musicdl/compare/{previous_tag}...{new_tag}
"""
```



## Implementation Details

### Changelog Structure

The structured changelog will follow this format:

```markdown
# Changelog

## [v0.14] - 2025-01-15

### ⚠️ Breaking Changes
- **Configuration**: Changed config file format from v1.1 to v1.2 (#123)

### 🔒 Security
- **Authentication**: Fixed credential exposure in logs (#456)

### Features
- **Downloader**: Added parallel download support (#789)
- **Docker**: Added slim image variant (#101)

### Bug Fixes
- **Configuration**: Fixed environment variable resolution (#234)
- **Downloader**: Fixed retry logic for failed downloads (#567)

### Documentation
- **README**: Updated installation instructions (#890)
- **Plans**: Added implementation plans for all TODO items (#111)

### Refactoring
- **Config**: Moved Spotify credentials to environment variables (#222)

### CI/CD
- **Workflows**: Updated Docker build workflow (#333)

---
**Contributors**: @user1, @user2
**Full Changelog**: https://github.com/sv4u/musicdl/compare/v0.13...v0.14
```



### Configuration File Location

- **`.cliff.toml`**: Root of repository
- **`CHANGELOG.md`**: Root of repository (generated/updated)

### Integration Points

1. **Release Workflow**: Generate changelog before creating release
2. **CHANGELOG.md**: Maintain persistent changelog file
3. **Git Tags**: Use tags to determine version ranges
4. **Conventional Commits**: Parse commit messages for categorization

## Testing Strategy

### Unit Tests

1. Test git-cliff configuration parsing
2. Test changelog generation from sample commits
3. Test breaking changes detection
4. Test scope-based grouping

### Integration Tests

1. Test changelog generation in dry-run mode
2. Test with various commit types and scopes
3. Test with breaking changes
4. Test with PR/issue links

### Manual Testing

1. Generate changelog locally with git-cliff
2. Review generated changelog format
3. Test release creation with new changelog
4. Verify CHANGELOG.md updates correctly

## Risk Assessment

### Low Risk

- git-cliff is well-maintained and stable
- Configuration is declarative and testable
- Can be tested in dry-run mode

### Medium Risk

- git-cliff installation in CI (mitigated by using action or pre-built binary)
- Configuration complexity (mitigated by starting simple and iterating)

### High Risk

- None identified

### Mitigation Strategies

1. **Test in Dry-Run**: Test changelog generation before actual releases
2. **Incremental Rollout**: Start with basic config, add features gradually
3. **Fallback**: Keep GitHub CLI as fallback if git-cliff fails
4. **Validation**: Validate generated changelog before creating release

## Success Criteria

1. ✅ Structured changelog with categorized sections
2. ✅ Breaking changes highlighted prominently
3. ✅ Scope-based grouping (config, downloader, docker, etc.)
4. ✅ PR and issue links included
5. ✅ CHANGELOG.md file maintained in repository
6. ✅ Security fixes section (if applicable)
7. ✅ All existing tests pass
8. ✅ Release workflow works with new changelog generation

## Rollback Plan

If issues are discovered:

1. Revert to GitHub CLI `--generate-notes`
2. Remove git-cliff installation
3. Remove `.cliff.toml` configuration
4. Investigate root cause
5. Create fix branch
6. Re-test and re-deploy

## Timeline

- **Day 1**: Research and choose tool, create `.cliff.toml` configuration
- **Day 2**: Implement changelog generation in workflow, test locally
- **Day 3**: Update release workflow, add CHANGELOG.md maintenance
- **Day 4**: Testing, documentation, final validation

## Dependencies

- `git-cliff` (or alternative tool)
- GitHub Actions (existing)
- Conventional Commits format (already in use)

## Related Files

- `.github/workflows/release-and-publish.yml` - Update changelog generation
- `.cliff.toml` - New configuration file
- `CHANGELOG.md` - New/updated changelog file
- `README.md` - May need to document changelog location

## Notes for Junior Engineers

### Why Structured Changelogs?

- **User Experience**: Easier to understand what changed
- **Communication**: Clear communication of changes to users
- **History**: Maintains project history and evolution
- **Compliance**: Some projects require structured changelogs

### Conventional Commits

The project uses Conventional Commits format:

- `type(scope): description`
- Types: feat, fix, docs, refactor, test, ci, chore
- Scopes: config, downloader, docker, ci, etc.

### git-cliff

- Tool for generating changelogs from git history
- Uses TOML configuration
- Supports templates and custom formatting
- Designed for Conventional Commits

### Common Pitfalls

1. **Configuration Errors**: Test `.cliff.toml` locally first
2. **Tag Range**: Ensure correct tag range for changelog
3. **Commit Format**: Ensure commits follow Conventional Commits
4. **Breaking Changes**: Mark breaking changes with `!` or `BREAKING CHANGE:`

### Debugging Tips

- Test locally: `git-cliff --tag v0.14 --latest`
- Check configuration: `git-cliff --check`
- Validate commits: Ensure they follow Conventional Commits format
- Review generated changelog before release

## Notes for Technical Management

### Business Impact

- **User Communication**: Better communication of changes to users
- **Professionalism**: More professional release documentation
- **Transparency**: Clear visibility into project changes
- **Compliance**: Meets requirements for structured changelogs

### Resource Requirements

- **Development Time**: 3-4 days
- **Testing Time**: 1 day
- **Risk**: Low (well-established tool, can test in dry-run)

### Recommendation