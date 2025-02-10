#!/bin/bash
# Define directory
HLS_DIR="/var/www/hls"

# Clean existing files
rm -f ${HLS_DIR}/*.ts ${HLS_DIR}/playlist.m3u8

# Create directory if it doesn't exist
mkdir -p ${HLS_DIR}

ffmpeg -loglevel info \
    -fflags nobuffer \
    -flags low_delay \
    -rtsp_transport tcp \
    -i "rtsp://[ip]/user=[user]&password=[password]&channel=[channel]&stream=0.sdp" \
    -an \
    -c:v libx264 \
    -preset ultrafast \
    -tune zerolatency \
    -profile:v baseline \
    -f hls \
    -s 1280x720 \
    -hls_time 2 \
    -hls_list_size 3 \
    -hls_flags delete_segments+independent_segments \
    -hls_segment_type mpegts \
    -flush_packets 1 \
    -hls_segment_filename "${HLS_DIR}/segment_%03d.ts" \
    "${HLS_DIR}/playlist.m3u8"
