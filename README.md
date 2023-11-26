# YouTube Playlist(Video) Splitter


Program to split YouTube videos (that are basically playlists just in one video) into multiple mp3 files. This whole thing works using yt-dlp, ffmpeg and jq. I made it for personal use so it's a bit clunky and not really user friendly.


## Cool features:


- Adds tags to all mp3 files based on video information
- Adds cover art using video thumbnail and cropping it to be 1:1 (using ffmpeg)
- Can accept txt files with multiple links to download in a batch or just one link
- Option to keep downloaded files (mp3 file of a video, cover arts and json dump from yt-dlp)


## To-Do


- Add more verbose errors
- Add a tmp folder to store all files that are needed
- Clean up code ... 500 lines in one files is too much
