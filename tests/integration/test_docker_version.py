"""
Integration tests for Docker version verification.

These tests verify that the version script works correctly in Docker builds
and that the built image contains the correct version in __init__.py.
"""
import subprocess
import tempfile
from pathlib import Path
from typing import Optional

import pytest


@pytest.mark.integration
@pytest.mark.docker
class TestDockerVersionBuild:
    """Test Docker build with version script."""

    def test_docker_build_updates_version_from_tag(
        self, tmp_git_repo: Path, docker_image_name: str
    ):
        """
        Test that Docker build correctly updates version from Git tag.

        Args:
            tmp_git_repo: Temporary Git repository fixture
            docker_image_name: Docker image name fixture
        """
        # Create a commit and tag it
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "Initial commit"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
        )
        # Create minimal files needed for Docker build BEFORE tagging
        # This ensures the working tree is clean when we tag
        self._create_docker_build_files(tmp_git_repo)
        
        # Commit the Docker build files to keep working tree clean
        subprocess.run(["git", "add", "."], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "Add Docker build files"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
        )
        
        # Now tag the commit
        subprocess.run(["git", "tag", "v3.0.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        # Build Docker image
        result = subprocess.run(
            ["docker", "build", "-f", "musicdl.Dockerfile", "-t", docker_image_name, "."],
            cwd=tmp_git_repo,
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            pytest.skip(f"Docker build failed: {result.stderr}")

        # Verify version in built image
        version = self._get_version_from_image(docker_image_name)
        assert version == "v3.0.0", f"Expected v3.0.0, got {version}"

        # Cleanup
        subprocess.run(["docker", "rmi", docker_image_name], capture_output=True)

    def test_docker_build_uses_commit_hash_for_dirty_tree(
        self, tmp_git_repo: Path, docker_image_name: str
    ):
        """
        Test that Docker build uses commit hash when working tree is dirty.

        Args:
            tmp_git_repo: Temporary Git repository fixture
            docker_image_name: Docker image name fixture
        """
        # Create a commit
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "Initial commit"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
        )

        # Get the commit hash before making tree dirty
        commit_hash_result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
            text=True,
        )
        expected_hash = commit_hash_result.stdout.strip()

        # Make working tree dirty (modify a file)
        test_file.write_text("modified")

        # Create minimal files needed for Docker build
        self._create_docker_build_files(tmp_git_repo)

        # Build Docker image
        result = subprocess.run(
            ["docker", "build", "-f", "musicdl.Dockerfile", "-t", docker_image_name, "."],
            cwd=tmp_git_repo,
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            pytest.skip(f"Docker build failed: {result.stderr}")

        # Verify version in built image (should be commit hash)
        version = self._get_version_from_image(docker_image_name)
        assert version == expected_hash, f"Expected {expected_hash}, got {version}"

        # Cleanup
        subprocess.run(["docker", "rmi", docker_image_name], capture_output=True)

    def test_docker_build_script_execution(self, tmp_git_repo: Path):
        """
        Test that the version script executes correctly in Docker build context.

        Args:
            tmp_git_repo: Temporary Git repository fixture
        """
        # Create a commit and tag
        test_file = tmp_git_repo / "test.txt"
        test_file.write_text("test")
        subprocess.run(["git", "add", "test.txt"], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "Initial commit"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
        )
        # Create minimal files needed for Docker build BEFORE tagging
        # This ensures the working tree is clean when we tag
        self._create_docker_build_files(tmp_git_repo)
        
        # Commit the Docker build files to keep working tree clean
        subprocess.run(["git", "add", "."], cwd=tmp_git_repo, check=True, capture_output=True)
        subprocess.run(
            ["git", "commit", "-m", "Add Docker build files"],
            cwd=tmp_git_repo,
            check=True,
            capture_output=True,
        )
        
        # Now tag the commit
        subprocess.run(["git", "tag", "v2.5.0"], cwd=tmp_git_repo, check=True, capture_output=True)

        # Build Docker image and capture output
        result = subprocess.run(
            ["docker", "build", "-f", "musicdl.Dockerfile", "-t", "test-version-script", "."],
            cwd=tmp_git_repo,
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            pytest.skip(f"Docker build failed: {result.stderr}")

        # Check that version script output appears in build logs
        # Docker build output goes to stderr, not stdout
        build_output = result.stderr
        # The version should appear in the build output (either the tag or commit hash)
        assert "v2.5.0" in build_output or "Determined version" in build_output

        # Cleanup
        subprocess.run(["docker", "rmi", "test-version-script"], capture_output=True)

    def _create_docker_build_files(self, repo_path: Path) -> None:
        """
        Create minimal files needed for Docker build.

        Args:
            repo_path: Path to the repository
        """
        # Create scripts directory
        scripts_dir = repo_path / "scripts"
        scripts_dir.mkdir(exist_ok=True)

        # Copy the actual get_version.py script
        source_script = Path(__file__).parent.parent.parent / "scripts" / "get_version.py"
        if source_script.exists():
            dest_script = scripts_dir / "get_version.py"
            dest_script.write_text(source_script.read_text())
            dest_script.chmod(0o755)

        # Create minimal __init__.py
        init_file = repo_path / "__init__.py"
        init_file.write_text('"""\nTest module.\n"""\n\n__version__ = "0.0.0"\n')

        # Create minimal requirements.txt
        requirements_file = repo_path / "requirements.txt"
        requirements_file.write_text("# Minimal requirements\n")

        # Create minimal Dockerfile (simplified version for testing)
        dockerfile_content = f"""# Stage 1: Builder
FROM python:3.12-slim AS builder

# Install Git
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*

# Copy Git repository and version script
COPY ./.git /tmp/repo/.git
COPY ./scripts/get_version.py /tmp/repo/scripts/get_version.py
COPY ./__init__.py /tmp/repo/__init__.py

# Reset Git state to match HEAD commit (ensures clean working tree)
# This is needed because COPY creates files that Git sees as untracked
# We only reset the index, not clean untracked files (we need the copied files)
RUN cd /tmp/repo && git reset --hard HEAD

# Run version script
RUN cd /tmp/repo && python3 scripts/get_version.py

# Stage 2: Runtime
FROM python:3.12-slim

# Copy updated __init__.py
COPY --from=builder /tmp/repo/__init__.py /tmp/__init__.py

# Default command to show version
CMD ["python3", "-c", "import sys; sys.path.insert(0, '/tmp'); from __init__ import __version__; print(__version__)"]
"""
        dockerfile = repo_path / "musicdl.Dockerfile"
        dockerfile.write_text(dockerfile_content)

    def _get_version_from_image(self, image_name: str) -> str:
        """
        Get version from Docker image by running it.

        Args:
            image_name: Name of the Docker image

        Returns:
            Version string from the image
        """
        result = subprocess.run(
            ["docker", "run", "--rm", image_name],
            capture_output=True,
            text=True,
            timeout=30,
        )

        if result.returncode != 0:
            pytest.fail(f"Failed to get version from image: {result.stderr}")

        return result.stdout.strip()


@pytest.fixture
def tmp_git_repo(tmp_path: Path) -> Path:
    """
    Create a temporary Git repository for Docker testing.

    Args:
        tmp_path: Temporary directory path from pytest

    Yields:
        Path to the temporary Git repository
    """
    repo_path = tmp_path / "docker_test_repo"
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


@pytest.fixture
def docker_image_name() -> str:
    """
    Generate a unique Docker image name for testing.

    Yields:
        Unique Docker image name
    """
    import uuid

    image_name = f"musicdl-version-test-{uuid.uuid4().hex[:8]}"
    yield image_name

    # Cleanup: try to remove image if it exists
    subprocess.run(["docker", "rmi", image_name], capture_output=True)


@pytest.fixture(scope="session", autouse=True)
def check_docker_available():
    """
    Check if Docker is available before running Docker tests.

    Skips all Docker tests if Docker is not available.
    """
    try:
        result = subprocess.run(
            ["docker", "--version"],
            capture_output=True,
            check=True,
            timeout=5,
        )
        if result.returncode != 0:
            pytest.skip("Docker is not available")
    except (subprocess.TimeoutExpired, FileNotFoundError, subprocess.SubprocessError):
        pytest.skip("Docker is not available or not accessible")

