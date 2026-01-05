"""
Unit tests for the get_version.py script.
"""
import subprocess
from pathlib import Path
from typing import Optional
from unittest.mock import patch, MagicMock

import pytest

# Import the functions we want to test
# Add scripts directory to path
import sys
from pathlib import Path

scripts_path = Path(__file__).parent.parent.parent / "scripts"
if str(scripts_path) not in sys.path:
    sys.path.insert(0, str(scripts_path))

# Import after adding to path
from get_version import (
    run_git_command,
    is_working_tree_dirty,
    get_latest_tag,
    get_current_tag,
    get_commit_hash,
    determine_version,
    update_init_file,
)


@pytest.mark.unit
class TestRunGitCommand:
    """Test run_git_command function."""

    def test_successful_command(self, tmp_path: Path):
        """Test successful Git command execution."""
        # Create a temporary Git repository
        subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
        subprocess.run(
            ["git", "config", "user.email", "test@example.com"],
            cwd=tmp_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(
            ["git", "config", "user.name", "Test User"],
            cwd=tmp_path,
            check=True,
            capture_output=True,
        )

        output, exit_code = run_git_command(["rev-parse", "--git-dir"], cwd=tmp_path)
        assert exit_code == 0
        assert ".git" in output

    def test_failed_command(self, tmp_path: Path):
        """Test failed Git command execution."""
        output, exit_code = run_git_command(["rev-parse", "nonexistent"], cwd=tmp_path)
        assert exit_code != 0

    def test_git_not_found(self):
        """Test behavior when Git is not found."""
        with patch("subprocess.run", side_effect=FileNotFoundError()):
            with pytest.raises(SystemExit):
                run_git_command(["version"])


@pytest.mark.unit
class TestIsWorkingTreeDirty:
    """Test is_working_tree_dirty function."""

    def test_clean_working_tree(self, tmp_git_repo: Path):
        """Test clean working tree detection."""
        # Create and commit a file to ensure we have a clean state
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)
        
        assert is_working_tree_dirty(cwd=tmp_git_repo) is False

    def test_dirty_working_tree(self, tmp_git_repo: Path):
        """Test dirty working tree detection."""
        # Create an uncommitted file
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test content")
        assert is_working_tree_dirty(cwd=tmp_git_repo) is True


@pytest.mark.unit
class TestGetLatestTag:
    """Test get_latest_tag function."""

    def test_no_tags(self, tmp_git_repo: Path):
        """Test when no tags exist."""
        tag = get_latest_tag(cwd=tmp_git_repo)
        assert tag is None

    def test_with_v_prefix_tag(self, tmp_git_repo: Path):
        """Test getting tag with v prefix."""
        # Create a commit first
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        # Create a tag with v prefix
        subprocess.run(["git", "tag", "v1.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        tag = get_latest_tag(cwd=tmp_git_repo)
        assert tag == "v1.0.0"

    def test_with_multiple_tags(self, tmp_git_repo: Path):
        """Test getting latest tag when multiple tags exist."""
        # Create commits and tags
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        subprocess.run(["git", "tag", "v1.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)
        test_file.write_text("test2")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Second commit"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "tag", "v2.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        tag = get_latest_tag(cwd=tmp_git_repo)
        assert tag == "v2.0.0"


@pytest.mark.unit
class TestGetCurrentTag:
    """Test get_current_tag function."""

    def test_not_on_tagged_commit(self, tmp_git_repo: Path):
        """Test when current commit is not tagged."""
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        tag = get_current_tag(cwd=tmp_git_repo)
        assert tag is None

    def test_on_tagged_commit(self, tmp_git_repo: Path):
        """Test when current commit is tagged."""
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "tag", "v1.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        tag = get_current_tag(cwd=tmp_git_repo)
        assert tag == "v1.0.0"


@pytest.mark.unit
class TestGetCommitHash:
    """Test get_commit_hash function."""

    def test_long_format(self, tmp_git_repo: Path):
        """Test getting long commit hash."""
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        commit_hash = get_commit_hash(long_format=True, cwd=tmp_git_repo)
        assert len(commit_hash) == 40  # SHA-1 is 40 characters
        assert commit_hash != "unknown"

    def test_short_format(self, tmp_git_repo: Path):
        """Test getting short commit hash."""
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        commit_hash = get_commit_hash(long_format=False, cwd=tmp_git_repo)
        assert len(commit_hash) == 7  # Short hash is 7 characters
        assert commit_hash != "unknown"

    def test_no_commit(self, tmp_path: Path):
        """Test getting commit hash when no commits exist."""
        # Create Git repo but don't commit
        subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
        subprocess.run(
            ["git", "config", "user.email", "test@example.com"],
            cwd=tmp_path,
            check=True,
            capture_output=True,
        )
        subprocess.run(
            ["git", "config", "user.name", "Test User"],
            cwd=tmp_path,
            check=True,
            capture_output=True,
        )

        commit_hash = get_commit_hash(long_format=True, cwd=tmp_path)
        assert commit_hash == "unknown"


@pytest.mark.unit
class TestDetermineVersion:
    """Test determine_version function."""

    def test_dirty_working_tree_uses_commit_hash(self, tmp_git_repo: Path):
        """Test that dirty working tree uses commit hash."""
        # Create a commit
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        # Make working tree dirty
        test_file.write_text("modified")
        version = determine_version(cwd=tmp_git_repo)
        assert len(version) == 40  # Should be long commit hash
        assert version != "unknown"

    def test_clean_tagged_commit_uses_tag(self, tmp_git_repo: Path):
        """Test that clean tagged commit uses tag."""
        # Create a commit and tag it
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "tag", "v2.1.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        version = determine_version(cwd=tmp_git_repo)
        assert version == "v2.1.0"

    def test_clean_untagged_commit_uses_latest_tag(self, tmp_git_repo: Path):
        """Test that clean untagged commit uses latest tag."""
        # Create commits and tags
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "tag", "v1.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        # Create another commit (not tagged)
        test_file.write_text("test2")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Second commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        version = determine_version(cwd=tmp_git_repo)
        assert version == "v1.0.0"  # Should use latest tag

    def test_no_tags_uses_commit_hash(self, tmp_git_repo: Path):
        """Test that when no tags exist, uses commit hash."""
        # Create a commit but no tags
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(["git", "commit", "-m", "Initial commit"], cwd=tmp_git_repo, check=True, capture_output=True)

        version = determine_version(cwd=tmp_git_repo)
        assert len(version) == 40  # Should be commit hash
        assert version != "unknown"


@pytest.mark.unit
class TestUpdateInitFile:
    """Test update_init_file function."""

    def test_update_existing_version(self, tmp_path: Path):
        """Test updating existing __version__ in __init__.py."""
        init_file = tmp_path / "__init__.py"
        init_file.write_text('"""\nTest module.\n"""\n\n__version__ = "1.0.0"\n')

        update_init_file("v2.1.0", init_file)

        content = init_file.read_text()
        assert '__version__ = "v2.1.0"' in content
        assert '__version__ = "1.0.0"' not in content

    def test_add_version_to_new_file(self, tmp_path: Path):
        """Test adding version to new __init__.py file."""
        init_file = tmp_path / "__init__.py"
        init_file.write_text('"""\nTest module.\n"""\n')

        update_init_file("v1.0.0", init_file)

        content = init_file.read_text()
        assert '__version__ = "v1.0.0"' in content

    def test_add_version_after_docstring(self, tmp_path: Path):
        """Test adding version after docstring."""
        init_file = tmp_path / "__init__.py"
        init_file.write_text('"""\nTest module.\n"""\n\n# Some comment\n')

        update_init_file("v1.0.0", init_file)

        content = init_file.read_text()
        assert '__version__ = "v1.0.0"' in content
        # Version should be after docstring
        lines = content.splitlines()
        version_index = next(i for i, line in enumerate(lines) if "__version__" in line)
        assert version_index > 2  # After docstring


# Fixtures for tests
@pytest.fixture
def tmp_git_repo(tmp_path: Path) -> Path:
    """
    Create a temporary Git repository for testing.

    Args:
        tmp_path: Temporary directory path from pytest

    Yields:
        Path to the temporary Git repository
    """
    repo_path = tmp_path / "test_repo"
    repo_path.mkdir()

    # Initialize Git repository
    subprocess.run(["git", "init"], cwd=repo_path, check=True, capture_output=True)
    subprocess.run(
        ["git", "config", "user.email", "test@example.com"],
        cwd=repo_path,
        check=True,
        capture_output=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "Test User"],
        cwd=repo_path,
        check=True,
        capture_output=True,
    )

    yield repo_path

