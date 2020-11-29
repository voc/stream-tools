#!/bin/sh
file="$1"
./transcoderload \
    -i "${file}" \
    -filter_complex "
        [0:v:0] split [hd1][hd2];
        [hd1] scale=1024:576,split [sd1][sd2];
        [hd2] framestep=step=500,split [poster][poster2];
        [poster2] scale=w=213:h=-1 [thumb]
    " \
    \
    -map '0:v:0' \
        -metadata:s:v:0 title="HD" \
        -c:v:0 libx264 \
            -maxrate:v:0 2800k \
            -crf:v:0 21 \
            -bufsize:v:0 5600k \
            -pix_fmt:v:0 yuv420p \
            -profile:v:0 main \
            -r:v:0 25 \
            -keyint_min:v:0 75 \
            -g:v:0 75 \
            -flags:v:0 +cgop \
            -preset:v:0 veryfast \
    \
    -map '[sd1]' \
        -metadata:s:v:1 title="SD" \
        -c:v:1 libx264 \
            -maxrate:v:1 800k \
            -crf:v:1 18 \
            -bufsize:v:1 3600k \
            -pix_fmt:v:1 yuv420p \
            -profile:v:1 main \
            -r:v:1 25 \
            -keyint_min:v:1 75 \
            -g:v:1 75 \
            -flags:v:1 +cgop \
            -preset:v:1 veryfast \
    \
    -map '0:v:1?' \
        -metadata:s:v:2 title="Slides" \
        -c:v:2 copy \
    \
    -c:a aac -ac:a 2 -b:a 128k \
    \
    -map '0:a:0' -metadata:s:a:0 title="Native" \
    -map '0:a:1?' -metadata:s:a:1 title="Translated" \
    -map '0:a:2?' -metadata:s:a:2 title="Translated-2" \
    \
    -max_muxing_queue_size 400 \
    -f matroska /dev/null \
    \
    -c:v libvpx-vp9 \
        -deadline:v realtime -cpu-used:v 8 \
        -frame-parallel:v 1 -crf:v 23 -row-mt:v 1 \
    \
        -map '0:v:0' \
        -metadata:s:v:0 title="HD" \
        -threads:v:0 4 \
        -tile-columns:v:0 2 \
        -r:v:0 25 \
        -keyint_min:v:0 75 \
        -g:v:0 75 \
        -b:v:0 2800k -maxrate:v:0 2800k \
        -bufsize:v:0 8400k \
    -map '[sd2]' \
        -metadata:s:v:1 title="SD" \
        -threads:v:1 4 \
        -tile-columns:v:1 1 \
        -r:v:1 25 \
        -keyint_min:v:1 75 \
        -g:v:1 75 \
        -b:v:1 800k -maxrate:v:1 800k \
        -bufsize:v:1 2400k \
    \
    -map '0:v:1?' \
        -metadata:s:v:2 title="Slides" \
        -threads:v:2 4 \
        -tile-columns:v:2 1 \
        -keyint_min:v:2 15 \
        -g:v:2 15 \
        -b:v:2 100k -maxrate:v:2 100k \
        -bufsize:v:2 750k \
    \
    -c:a libopus -ac:a 2 -b:a 128k \
    -af "aresample=async=1:min_hard_comp=0.100000:first_pts=0" \
    \
    -map '0:a:0' -metadata:s:a:0 title="Native" \
    -map '0:a:1?' -metadata:s:a:1 title="Translated" \
    -map '0:a:2?' -metadata:s:a:2 title="Translated-2" \
    \
    -fflags +genpts \
    -max_muxing_queue_size 1000 \
    -f matroska /dev/null \
    \
    -c:v mjpeg -pix_fmt:v yuvj420p \
    -map '[poster]' \
        -metadata:s:v:0 title="Poster" \
    -map '[thumb]' \
        -metadata:s:v:1 title="Thumbnail" \
    -an \
    \
    -fflags +genpts \
    -max_muxing_queue_size 400 \
    -f matroska /dev/null \
    \
    -map '0:a:0' \
        -c:a:0 libmp3lame -b:a:0 96k \
        -metadata:s:a:0 title="Native" \
    \
    -map '0:a:0' \
        -c:a:1 libopus -vbr:a:1 off -b:a:1 32k \
        -metadata:s:a:1 title="Native" \
    \
    \
    -map '0:a:1?' \
        -c:a:2 libmp3lame -b:a:2 96k \
        -metadata:s:a:2 title="Translated" \
    \
    -map '0:a:1?' \
        -c:a:3 libopus -vbr:a:3 off -b:a:3 32k \
        -metadata:s:a:3 title="Translated" \
    \
    \
    -map '0:a:2?' \
        -c:a:4 libmp3lame -b:a:4 96k \
        -metadata:s:a:4 title="Translated-2" \
    \
    -map '0:a:2?' \
        -c:a:5 libopus -vbr:a:5 off -b:a:5 32k \
        -metadata:s:a:5 title="Translated-2" \
    \
    -max_muxing_queue_size 400 \
    -f matroska /dev/null
