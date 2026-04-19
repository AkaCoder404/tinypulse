# Deploying and Updating TinyPulse

This guide covers how to deploy TinyPulse for production use and how to safely update it when new versions are released.

## Deployment (Systemd)

The easiest way to run TinyPulse persistently on a Linux VPS (like DigitalOcean, Linode, or AWS) is by running it as a background service using `systemd`. 

We have provided an automated deployment script that will:
1. Download the latest release binary.
2. Create a dedicated installation directory (`/opt/tinypulse`).
3. Securely configure your admin password.
4. Set up and start a `systemd` service that runs TinyPulse in the background and restarts it automatically on server reboots.

### Running the Setup Script

You can find the deployment script in the repository at `scripts/setup_systemctl.sh`.

Upload the script to your server, make it executable, and run it as root:

```bash
chmod +x setup_systemctl.sh
sudo ./setup_systemctl.sh
```

Follow the prompt to set your `admin` password. Once the script finishes, TinyPulse will be running on `http://localhost:8080`.

---

## Updating TinyPulse

Because TinyPulse is a single static binary, updating is incredibly simple. 

### 1. Stop the existing service
```bash
sudo systemctl stop tinypulse
```

### 2. Replace the binary
Download the new binary from the [GitHub Releases](https://github.com/AkaCoder404/tinypulse/releases) page and overwrite your existing binary.

```bash
# Example for replacing the binary in /opt/tinypulse
wget -O /opt/tinypulse/tinypulse https://github.com/AkaCoder404/tinypulse/releases/latest/download/tinypulse-linux-amd64
chmod +x /opt/tinypulse/tinypulse
```

### 3. Restart the service
```bash
sudo systemctl start tinypulse
```

---

## Database Migrations & Rollbacks

When you start a newer version of TinyPulse, it automatically handles any required database upgrades (e.g., if a new feature requires a new column in the database).

**Data Safety:** Before making *any* changes to your database schema, TinyPulse creates a safe, point-in-time backup in the same directory (e.g., `uptime_backup_v1.db`).

### How to Rollback

If a new version has a bug or an upgrade fails, you can safely revert your data:

1. Stop the TinyPulse service:
   ```bash
   sudo systemctl stop tinypulse
   ```
2. Delete the partially-upgraded database:
   ```bash
   rm /opt/tinypulse/uptime.db
   ```
3. Restore the automated backup:
   ```bash
   mv /opt/tinypulse/uptime_backup_v1.db /opt/tinypulse/uptime.db
   ```
4. Replace the new binary with the older version you were previously running, and restart the service.