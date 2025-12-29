# TrueNAS Scale Deployment Guide for musicdl

This guide provides comprehensive instructions for deploying musicdl on TrueNAS Scale using multiple deployment methods.

## Prerequisites

Before deploying musicdl on TrueNAS Scale, ensure you have:

1. **TrueNAS Scale** installed and running
2. **Docker** or **Kubernetes** access (depending on deployment method)
3. **Spotify API Credentials**:
   - Client ID
   - Client Secret
   - (Get these from [Spotify Developer Dashboard](https://developer.spotify.com/dashboard))

4. **Storage Setup**:
   - A dataset or directory for storing downloaded music
   - Recommended: Create a dedicated dataset (e.g., `/mnt/pool/datasets/music`)

5. **Network Access**:
   - Container needs internet access to download from YouTube/Spotify
   - No inbound ports required

## Deployment Methods

### Method 1: Helm Chart (Recommended)

The Helm chart provides a production-ready Kubernetes Job with proper resource management and configuration options.

**Note**: This chart uses a Kubernetes `Job` resource, not a `Deployment`. This is because musicdl runs once, downloads music, and exits. Jobs are designed for run-once workloads, while Deployments are for long-running services that would continuously restart.

#### Step 1: Build the Docker Image

First, build the musicdl Docker image on your TrueNAS Scale system or push it to a registry:

```bash
# On TrueNAS Scale or build machine
cd /path/to/musicdl
docker build -f musicdl.Dockerfile -t musicdl:latest .
```

If using a registry:

```bash
docker tag musicdl:latest your-registry/musicdl:latest
docker push your-registry/musicdl:latest
```

#### Step 2: Prepare Configuration

Create or update the `values.yaml` file in the Helm chart directory:

```yaml
# truenas-scale/helm/musicdl/values.yaml

image:
  repository: musicdl  # or your-registry/musicdl
  tag: latest
  pullPolicy: IfNotPresent

volumes:
  download:
    enabled: true
    hostPath: /mnt/pool/datasets/music  # Update to your path
    mountPath: /download

  # Optional: Override config
  config:
    enabled: true
    hostPath: /mnt/pool/datasets/musicdl/config.yaml
    mountPath: /scripts/config.yaml

resources:
  limits:
    cpu: 2
    memory: 2Gi
  requests:
    cpu: 1
    memory: 1Gi
```

#### Step 3: Install with Helm

```bash
# Install the chart
helm install musicdl ./truenas-scale/helm/musicdl \
  --namespace musicdl \
  --create-namespace \
  --values ./truenas-scale/helm/musicdl/values.yaml

# Or install with custom values
helm install musicdl ./truenas-scale/helm/musicdl \
  --set volumes.download.hostPath=/mnt/pool/datasets/music
```

#### Step 4: Verify Job

```bash
# Check job status
kubectl get jobs -n musicdl

# Check pod status
kubectl get pods -n musicdl

# View logs
kubectl logs -n musicdl -l app.kubernetes.io/name=musicdl

# View job details
kubectl describe job -n musicdl musicdl
```

#### Running the Job Again

Since this is a Job (run-once), you need to delete and recreate it to run again:

```bash
# Delete the completed job
kubectl delete job -n musicdl musicdl

# Reinstall to run again
helm install musicdl ./truenas-scale/helm/musicdl \
  --namespace musicdl \
  --values ./truenas-scale/helm/musicdl/values.yaml
```

#### Updating the Job

```bash
# Update values and upgrade
helm upgrade musicdl ./truenas-scale/helm/musicdl \
  --values ./truenas-scale/helm/musicdl/values.yaml

# Or update specific values
helm upgrade musicdl ./truenas-scale/helm/musicdl \
  --set volumes.download.hostPath=/new/path
```

#### Uninstalling

```bash
helm uninstall musicdl -n musicdl
```

### Method 2: Docker Compose

Docker Compose provides a simpler deployment option, especially useful for single-node TrueNAS Scale setups.

#### Step 1: Prepare docker-compose.yml

Copy the provided `docker-compose.yml` file to your TrueNAS Scale system. **Important**: The compose file must be run from the `truenas-scale/` directory, as it uses `context: ..` to reference the parent directory where the Dockerfile and source files are located.

Update the volume paths in the compose file:

```yaml
volumes:
  - /mnt/pool/datasets/music:/download:rw  # Update this path
```

**Note**: The build context is set to `..` (parent directory) because the compose file is in `truenas-scale/` but the Dockerfile and source files are in the project root.

#### Step 2: Deploy via TrueNAS Scale UI

1. Navigate to **Apps** in TrueNAS Scale
2. Click **Discover Apps** or **Custom App**
3. Select **Launch Docker Image** or use **Custom App**
4. Configure:
   - **Image**: `musicdl:latest` (or your registry path)
   - **Volumes**: Add volume mount for download directory
   - **Environment Variables**: Set `CONFIG_PATH` if needed
   - **Resource Limits**: Configure CPU/Memory limits
   - **Restart Policy**: Set to `no` (the container runs once and exits)

#### Step 3: Deploy via Command Line

```bash
# Navigate to truenas-scale directory (important: compose file uses context: ..)
cd /path/to/musicdl/truenas-scale

# Start the container (runs once and exits)
docker compose -f docker-compose.yml up

# Or run in detached mode
docker compose -f docker-compose.yml up -d

# View logs
docker compose -f docker-compose.yml logs -f

# Remove the container after completion
docker compose -f docker-compose.yml down
```

**Note**: The container runs once, downloads music, and exits. It will not automatically restart. To run again, execute `docker compose up` again.

### Method 3: Manual TrueNAS Scale App Setup

For users who prefer the TrueNAS Scale web UI:

#### Step 1: Build and Push Image

```bash
docker build -f musicdl.Dockerfile -t musicdl:latest .
# Optionally push to a registry
```

#### Step 2: Create Custom App

1. Go to **Apps** → **Available Applications**
2. Click **Custom App** or **Launch Docker Image**
3. Fill in the following:

   **Basic Configuration:**
   - **Application Name**: `musicdl`
   - **Image Repository**: `musicdl:latest` (or your registry)
   - **Image Tag**: `latest`
   - **Update Policy**: `Recreate` or `Rolling`

   **Container Configuration:**
   - **Container Arguments**: `/scripts/config.yaml`
   - **Working Directory**: `/download`

   **Storage:**
   - **Host Path**: `/mnt/pool/datasets/music` (your music directory)
   - **Mount Path**: `/download`
   - **Access Mode**: `ReadWrite`

   **Optional - Config Override:**
   - Add another volume:
     - **Host Path**: `/mnt/pool/datasets/musicdl/config.yaml`
     - **Mount Path**: `/scripts/config.yaml`
     - **Access Mode**: `ReadOnly`

   **Resources:**
   - **CPU Limit**: `2` cores
   - **Memory Limit**: `2Gi`
   - **CPU Reservation**: `1` core
   - **Memory Reservation**: `1Gi`

   **Advanced:**
   - **Restart Policy**: `No` (container runs once and exits)
   - **Security Context**: 
     - **Run as Non-Root**: `true`
     - **User ID**: `1000`
     - **Group ID**: `1000`

4. Click **Save** to deploy

## Configuration

### Using Default Config

The Docker image includes a default `config.yaml` file. To use it, no additional configuration is needed.

### Overriding Config

#### Option 1: Volume Mount

Mount your custom `config.yaml` file:

**Docker Compose:**
```yaml
volumes:
  - /path/to/your/config.yaml:/scripts/config.yaml:ro
```

**Helm Chart:**
```yaml
volumes:
  config:
    enabled: true
    hostPath: /mnt/pool/datasets/musicdl/config.yaml
    mountPath: /scripts/config.yaml
```

#### Option 2: Environment Variable

Set `CONFIG_PATH` environment variable:

```yaml
environment:
  - CONFIG_PATH=/custom/path/config.yaml
```

Then mount the config file at that path.

### Config File Location on TrueNAS Scale

Recommended structure:

```
/mnt/pool/datasets/
├── music/              # Download directory
│   └── [downloaded files]
└── musicdl/           # Configuration directory
    └── config.yaml     # Custom config (optional)
```

## Volume Setup

### Creating a Dataset for Music

1. Go to **Storage** → **Pools**
2. Select your pool
3. Click **Add Dataset**
4. Set:
   - **Name**: `music`
   - **Type**: `Filesystem`
   - **Share Type**: `Generic`
5. Set permissions:
   - **User**: Your TrueNAS user
   - **Group**: Your TrueNAS group
   - **Mode**: `755` or `775`

### Setting Permissions

```bash
# Set ownership
chown -R user:group /mnt/pool/datasets/music

# Set permissions
chmod -R 755 /mnt/pool/datasets/music
```

## Monitoring and Logs

### Viewing Logs

**Helm/Kubernetes:**
```bash
kubectl logs -n musicdl -l app.kubernetes.io/name=musicdl -f
```

**Docker Compose:**
```bash
docker compose -f truenas-scale/docker-compose.yml logs -f
```

**Docker:**
```bash
docker logs musicdl -f
```

### Monitoring Downloads

Check the download directory:

```bash
ls -lh /mnt/pool/datasets/music
```

The container will create subdirectories based on your config's output template (e.g., `{artist}/{album}/`).

## Troubleshooting

### Container Won't Start

1. **Check logs**:
   ```bash
   docker logs musicdl
   # or
   kubectl logs -n musicdl <pod-name>
   ```

2. **Verify image exists**:
   ```bash
   docker images | grep musicdl
   ```

3. **Check volume permissions**:
   ```bash
   ls -la /mnt/pool/datasets/music
   ```

### Downloads Not Working

1. **Verify Spotify credentials** in config.yaml
2. **Check internet connectivity** from container
3. **Review logs** for specific error messages
4. **Verify volume mount** is writable:
   ```bash
   docker exec musicdl touch /download/test.txt
   ```

### Permission Issues

If you see permission errors:

1. **Check volume ownership**:
   ```bash
   ls -la /mnt/pool/datasets/music
   ```

2. **Update container user** in docker-compose.yml or Helm values:
   ```yaml
   user: "1000:1000"  # Match your TrueNAS user/group IDs
   ```

3. **Fix permissions**:
   ```bash
   chown -R 1000:1000 /mnt/pool/datasets/music
   chmod -R 755 /mnt/pool/datasets/music
   ```

### Config Not Loading

1. **Verify config path**:
   ```bash
   docker exec musicdl ls -la /scripts/config.yaml
   ```

2. **Check environment variable**:
   ```bash
   docker exec musicdl env | grep CONFIG_PATH
   ```

3. **Validate YAML syntax**:
   ```bash
   python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"
   ```

## Best Practices

1. **Resource Limits**: Set appropriate CPU and memory limits based on your system capacity
2. **Storage**: Use dedicated datasets for music downloads
3. **Backups**: Regularly backup your `config.yaml` file
4. **Monitoring**: Set up log aggregation if running multiple instances
5. **Updates**: Regularly update the Docker image for security patches
6. **Security**: 
   - Run containers as non-root user
   - Use read-only mounts for config files
   - Limit container capabilities

## Scheduling Downloads

The container runs once and exits after completing downloads. To schedule regular downloads:

### Using Cron (TrueNAS Scale)

1. Create a script:
   ```bash
   #!/bin/bash
   docker start musicdl
   ```

2. Add to crontab:
   ```bash
   # Run daily at 2 AM
   0 2 * * * /path/to/script.sh
   ```

### Using Kubernetes CronJob

Create a `CronJob` resource:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: musicdl-scheduler
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: musicdl
            image: musicdl:latest
            volumeMounts:
            - name: download
              mountPath: /download
          volumes:
          - name: download
            hostPath:
              path: /mnt/pool/datasets/music
          restartPolicy: Never
```

## Additional Resources

- [Main README](../README.md) - General musicdl documentation
- [Configuration Guide](../README.md#configuration) - Config file reference
- [TrueNAS Scale Documentation](https://www.truenas.com/docs/) - Official TrueNAS docs

## Support

For issues specific to:
- **musicdl functionality**: Check the main [README](../README.md)
- **Docker deployment**: Review Docker logs and this guide
- **TrueNAS Scale**: Consult [TrueNAS Documentation](https://www.truenas.com/docs/)

