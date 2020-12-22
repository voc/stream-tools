# ffmpeg wrapper app
Currently used for load-testing

## Usage
```
# sudo ./transcoderload -cmd /opt/transcoder/scripts/transcode.py -- --stream q1 --source http://deinemutter.fem.tu-ilmenau.de:8000/multi_schleife --sink foo -o null
```
produces an output like the following:
```
...
20/12/22 20:00:13 Confidence per job count: [1 26.62890625 28.52734375 32.171875 -15.921875 -8.125]
```

This means that 4 transcodings can be run on this machine with high confidence (for the used content).