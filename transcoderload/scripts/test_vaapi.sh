#!/bin/sh
file="$1"
#export LIBVA_DRIVER_NAME="iHD"
export LIBVA_DRIVER_NAME="i965"
./transcoderload \
    -init_hw_device vaapi=transcode:/dev/dri/renderD128 \
    -hwaccel vaapi \
    -hwaccel_output_format vaapi \
    -hwaccel_device transcode \
    -i "${file}" \
    -filter_hw_device transcode \
    -filter_complex "
        [0:v:0] hwupload,split=3 [hd1][_hd2][_hd3];
        [_hd2] hwdownload [hd2];
        [hd1] scale_vaapi=1024:576,split [sd1][sd2];
        [_hd3] framestep=step=500,split [poster1][poster2];
        [poster2] scale_vaapi=w=213:h=-1 [thumb]
    " \
    \
    -map '0:v:0' \
        -metadata:s:v:0 title="HD" \
        -c:v:0 h264_vaapi \
            -r:v:0 25 \
            -keyint_min:v:0 75 \
            -g:v:0 75 \
            -b:v:0 2800k \
            -bufsize:v:0 8400k \
            -flags:v:0 +cgop \
    -map '[sd1]' \
        -metadata:s:v:1 title="SD" \
        -c:v:1 h264_vaapi \
            -r:v:1 25 \
            -keyint_min:v:1 75 \
            -g:v:1 75 \
            -b:v:1 800k \
            -bufsize:v:1 2400k \
            -flags:v:1 +cgop \
    \
    -map '0:v:1?' \
        -metadata:s:v:2 title="Slides" \
        -c:v:2 copy \
    \
    -c:a copy \
    \
    -map '0:a:0' -metadata:s:a:0 title="Native" \
    -map '0:a:1?' -metadata:s:a:1 title="Translated" \
    -map '0:a:2?' -metadata:s:a:2 title="Translated-2" \
    \
    -fflags +genpts \
    -max_muxing_queue_size 400 \
    -f matroska \
    -content_type video/webm \
    /dev/null \
    \
    -map '[hd2]' \
        -metadata:s:v:0 title="HD" \
        -c:v:0 libvpx-vp9 \
            -deadline:v:0 realtime \
            -cpu-used:v:0 8 \
            -threads:v:0 4 \
            -frame-parallel:v:0 1 \
            -tile-columns:v:0 2 \
            \
            -r:v:0 25 \
            -keyint_min:v:0 75 \
            -g:v:0 75 \
            -crf:v:0 23 \
            -row-mt:v:0 1 \
            -b:v:0 2800k -maxrate:v:0 2800k \
            -bufsize:v:0 8400k \
\
    -map '[sd2]' \
        -metadata:s:v:1 title="SD" \
        -c:v:1 vp9_vaapi \
            -r:v:1 25 \
            -keyint_min:v:1 75 \
            -g:v:1 75 \
            -b:v:1 800k \
            -bufsize:v:1 2400k \
    \
    -map '0:v:1?' \
        -metadata:s:v:2 title="Slides" \
        -c:v:2 vp9_vaapi \
            -keyint_min:v:2 15 \
            -g:v:2 15 \
            -b:v:2 100k \
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
    -max_muxing_queue_size 400 \
    -f matroska \
    -content_type video/webm \
    /dev/null \
    \
    -c:v mjpeg_vaapi \
    -map '[poster1]' \
        -metadata:s:v:0 title="Poster" \
    -map '[thumb]' \
        -metadata:s:v:1 title="Thumbnail" \
    \
    -fflags +genpts \
    -max_muxing_queue_size 400 \
    -f matroska \
    -content_type video/webm \
    /dev/null \
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
    -f matroska \
    -content_type video/webm \
    /dev/null
