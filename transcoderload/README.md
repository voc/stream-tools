# transcoderload - ffmpeg wrapper
Used for testing how many instances of an ffmpeg commandline can be run in parallel on a single machine.

## Usage

### Run ffmpeg directly
In the easiest form the tool can be used to see how many realtime ffmpeg instances can be run in parallel before stalling occurs.

Just pass the ffmpeg options directly after double dashes (--) to indicate that the options for transcoderload have ended.
```sh
go build
./transcoderload -- -i <livestream-source> -c:v libx264 -f null -
```

### Run ffmpeg through a wrapper script
In case you use your own streaming-script which sets the ffmpeg options (like we do), you can also set the command to be run. The options then obviously depend upon your streaming script.

**Important:** Your script has to allow passing through the *-progress* option to ffmpeg because the tool uses this option to detect stalls.

```
./transcoderload -cmd /opt/transcoder/scripts/transcode.py -- --stream q1 --source <livestream-source> --sink foo -o null
```
produces an output like the following:
```
...
20/12/22 20:00:13 Confidence per job count: [1 26.62890625 28.52734375 32.171875 -15.921875 -8.125]
```

This means that 4 transcodings can be run on this machine with high confidence (for the used content).

## How it works
The tool runs a number of ffmpeg processes (jobs) which report their status and speed to http endpoints exposed by the tool on unix domain sockets. If the jobs run stable for a certain amount of time the current configuration is deemed stable and the number of jobs is increased for the next run.
If the transcoding speed drops below 1 for a considerable amount of time the job is considered stalled and the next run will start fewer jobs.
For each detected stall the duration required for a stable result will also be increased. So once a certain limit has been found it is tested more and more thouroughly.

For each number of jobs a "confidence rating" is determined by the app. It is calculated by adding/subtracting a "confidence value" to the current bucket depending on success/failure of the test. The value increases linearly with the test duration.