```text
* * * * * /path/to/command
│ │ │ │ │
│ │ │ │ └── day of week (0-7, 0 and 7 = Sunday)
│ │ │ └──── month (1-12)
│ │ └────── day of month (1-31)
│ └──────── hour (0-23)
└────────── minute (0-59)
```

# Common Examples

```bash
# Every minute
* * * * * /opt/scripts/healthcheck.sh

# Every 5 minutes
*/5 * * * * /opt/scripts/healthcheck.sh

# Every day at 2am
0 2 * * * /opt/scripts/backup.sh

# Every Monday at 9am
0 9 * * 1 /opt/scripts/weekly_report.sh

# Every hour on weekdays
0 * * * 1-5 /opt/scripts/sync.sh

# First day of every month at midnight
0 0 1 * * /opt/scripts/monthly_cleanup.sh
```