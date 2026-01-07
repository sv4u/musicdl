#!/usr/bin/env python3
"""
Script to determine version from Git tags and update __init__.py.

This script:
- Gets the latest Git tag (with v prefix) if on a clean, tagged commit
- Uses the long commit hash if the working tree is dirty
- Updates __init__.py with the determined version
"""

import subprocess
import sys
from pathlib import Path
from typing import Optional


def run_git_command(cmd: list[str], cwd: Optional[Path] = None) -> tuple[str, int]:
    """
    Run a Git command and return the output and exit code.

    Args:
        cmd: Git command as a list of strings
        cwd: Working directory for the command

    Returns:
        Tuple of (output, exit_code)
    """
    try:
        # Convert Path to string if provided
        cwd_str = str(cwd) if cwd else None
        result = subprocess.run(
            ["git"] + cmd,
            capture_output=True,
            text=True,
            cwd=cwd_str,
            check=False,
        )
        return result.stdout.strip(), result.returncode
    except FileNotFoundError:
        print("Error: Git is not installed or not in PATH", file=sys.stderr)
        sys.exit(1)


def is_working_tree_dirty(cwd: Optional[Path] = None) -> bool:
    """
    Check if the Git working tree has uncommitted changes.

    Args:
        cwd: Working directory for the command

    Returns:
        True if working tree is dirty, False otherwise
    """
    output, exit_code = run_git_command(["diff-index", "--quiet", "HEAD", "--"], cwd=cwd)
    # Exit code 0 means no differences (clean), non-zero means differences (dirty)
    return exit_code != 0


def get_latest_tag(cwd: Optional[Path] = None) -> Optional[str]:
    """
    Get the latest Git tag with v prefix.

    Args:
        cwd: Working directory for the command

    Returns:
        Latest tag with v prefix, or None if no tags exist
    """
    # First try to get v-prefixed tags
    output, exit_code = run_git_command(
        ["describe", "--tags", "--abbrev=0", "--match", "v*"], cwd=cwd
    )
    if exit_code == 0 and output:
        return output

    # Fallback to any tag
    output, exit_code = run_git_command(
        ["describe", "--tags", "--abbrev=0"], cwd=cwd
    )
    if exit_code == 0 and output:
        return output

    return None


def get_current_tag(cwd: Optional[Path] = None) -> Optional[str]:
    """
    Get the tag for the current commit, if it exists.

    Args:
        cwd: Working directory for the command

    Returns:
        Tag name if current commit is tagged, None otherwise
    """
    output, exit_code = run_git_command(
        ["describe", "--tags", "--exact-match", "HEAD"], cwd=cwd
    )
    if exit_code == 0 and output:
        return output
    return None


def get_commit_hash(long_format: bool = True, cwd: Optional[Path] = None) -> str:
    """
    Get the current commit hash.

    Args:
        long_format: If True, return full hash (40 chars), else short (7 chars)
        cwd: Working directory for the command

    Returns:
        Commit hash
    """
    # git rev-parse doesn't use format strings - use --short for short hash
    cmd = ["rev-parse", "HEAD"] if long_format else ["rev-parse", "--short", "HEAD"]
    output, exit_code = run_git_command(cmd, cwd=cwd)
    if exit_code == 0 and output:
        return output
    return "unknown"


def determine_version(cwd: Optional[Path] = None) -> str:
    """
    Determine the version from Git tags or commit hash.

    Args:
        cwd: Working directory for the command

    Returns:
        Version string (tag with v prefix, or commit hash for dirty trees)
    """
    # Check if working tree is dirty
    if is_working_tree_dirty(cwd=cwd):
        # Use long commit hash for dirty working tree
        commit_hash = get_commit_hash(long_format=True, cwd=cwd)
        return commit_hash

    # Check if current commit is tagged
    current_tag = get_current_tag(cwd=cwd)
    if current_tag:
        return current_tag

    # Otherwise, use the latest tag
    latest_tag = get_latest_tag(cwd=cwd)
    if latest_tag:
        return latest_tag

    # Fallback to commit hash if no tags exist
    commit_hash = get_commit_hash(long_format=True, cwd=cwd)
    return commit_hash


def update_init_file(version: str, init_file: Path) -> None:
    """
    Update __init__.py with the determined version.

    Args:
        version: Version string to write
        init_file: Path to __init__.py file
    """
    # Read existing file
    if init_file.exists():
        content = init_file.read_text()
    else:
        content = '"""\nmusicdl - Personal music downloader.\n"""\n\n'

    # Update or add __version__ line
    lines = content.splitlines()
    version_line = f'__version__ = "{version}"'

    # Find and replace existing __version__ line, or add it
    version_found = False
    for i, line in enumerate(lines):
        if line.startswith("__version__"):
            lines[i] = version_line
            version_found = True
            break

    if not version_found:
        # Add version line after docstring
        # Find the end of the docstring (empty line after """)
        insert_index = 0
        for i, line in enumerate(lines):
            if line.strip() == '"""' and i > 0:
                # Find the next empty line or end of file
                for j in range(i + 1, len(lines)):
                    if not lines[j].strip():
                        insert_index = j + 1
                        break
                else:
                    insert_index = len(lines)
                break

        lines.insert(insert_index, version_line)
        if insert_index < len(lines) - 1 and lines[insert_index + 1]:
            lines.insert(insert_index + 1, "")

    # Write updated content
    init_file.write_text("\n".join(lines) + "\n")


def main() -> int:
    """
    Main entry point for the version script.

    Returns:
        Exit code (0 for success, non-zero for error)
    """
    # Check for version override argument (for CI builds)
    version_override = None
    if len(sys.argv) > 1:
        version_override = sys.argv[1]
        if version_override in ["-h", "--help"]:
            print("Usage: get_version.py [VERSION_OVERRIDE]")
            print("  If VERSION_OVERRIDE is provided, use it directly.")
            print("  Otherwise, determine version from Git tags.")
            return 0

    # Determine the repository root (where .git directory is)
    # Use absolute path to avoid issues with working directory
    script_dir = Path(__file__).resolve().parent
    repo_root = script_dir.parent.resolve()

    # Use override if provided, otherwise determine from Git
    if version_override:
        version = version_override
        print(f"Using provided version override: {version}", file=sys.stderr)
    else:
        version = determine_version(cwd=repo_root)
        print(f"Determined version: {version}", file=sys.stderr)

    # Update __init__.py
    init_file = repo_root / "__init__.py"
    update_init_file(version, init_file)
    print(f"Updated {init_file} with version: {version}", file=sys.stderr)

    # Also print version to stdout for potential use in build scripts
    print(version)
    return 0


if __name__ == "__main__":
    sys.exit(main())

