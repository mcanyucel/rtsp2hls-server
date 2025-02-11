<!-- StreamPlayer.svelte -->
<script lang="ts">
    import { onDestroy } from 'svelte';
    import Hls from 'hls.js';
    import { Play } from 'lucide-svelte';

    export let text: string = 'Kamera';
    export let baseUrl: string = 'https://stream.test.example.com';
    
    let status: 'idle' | 'initializing' | 'loading-player' | 'ready' | 'error' = 'idle';
    let errorMessage: string = '';
    let videoElement: HTMLVideoElement;
    let hls: Hls | null = null;
    let heartbeatInterval: NodeJS.Timeout | null = null;

    const HEARTBEAT_INTERVAL = 60000;
    const INITIAL_DELAY = 5000;
    const POLL_INTERVAL = 2000;
    const MAX_POLL_ATTEMPTS = 15; // 30 seconds total polling time

    async function checkPlaylistReady(): Promise<boolean> {
        try {
            const response = await fetch(`${baseUrl}/playlist.m3u8`);
            if (!response.ok) return false;
            
            const content = await response.text();
            // Check if the playlist contains any .ts files
            return content.includes('.ts');
        } catch (error) {
            console.log('Playlist check failed:', error);
            return false;
        }
    }

    async function waitForPlaylist(): Promise<void> {
        console.log('Waiting for initial delay...');
        await new Promise(resolve => setTimeout(resolve, INITIAL_DELAY));
        
        console.log('Starting to poll playlist...');
        let attempts = 0;
        
        while (attempts < MAX_POLL_ATTEMPTS) {
            console.log(`Checking playlist (attempt ${attempts + 1}/${MAX_POLL_ATTEMPTS})...`);
            if (await checkPlaylistReady()) {
                console.log('Playlist is ready!');
                return;
            }
            
            await new Promise(resolve => setTimeout(resolve, POLL_INTERVAL));
            attempts++;
        }
        
        throw new Error('Timeout waiting for stream to become ready');
    }

    async function initializeStream() {
        try {
            status = 'initializing';
            errorMessage = '';

            if (!Hls.isSupported()) {
                throw new Error('HLS is not supported in your browser');
            }

            const response = await fetch(`${baseUrl}/api/initiate`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'application/json'
                }
            });

            const data = await response.json();
            console.log('Initiate response:', data);

            if (data.status === 'success') {
                console.log('Stream initialization successful, waiting for stream...');
                await waitForPlaylist();
                status = 'loading-player';
                await new Promise(resolve => setTimeout(resolve, 100));
                await startPlayer();
                startHeartbeat();
            } else {
                throw new Error(data.message || 'Failed to initialize stream');
            }
        } catch (error) {
            console.error('Stream initialization error:', error);
            handleError(error instanceof Error ? error.message : 'Unknown error occurred');
        }
    }

    async function startPlayer() {
        return new Promise<void>((resolve, reject) => {
            let retryCount = 0;
            const maxRetries = 5;
            
            const tryInitPlayer = () => {
                if (!videoElement) {
                    if (retryCount < maxRetries) {
                        retryCount++;
                        console.log(`Video element not found, retry ${retryCount}/${maxRetries}`);
                        setTimeout(tryInitPlayer, 200);
                        return;
                    }
                    reject(new Error('Video element not found after retries'));
                    return;
                }

                try {
                    cleanup();

                    hls = new Hls({
                        debug: false,
                        enableWorker: true,
                        lowLatencyMode: true,
                        backBufferLength: 90
                    });

                    const streamUrl = `${baseUrl}/playlist.m3u8`;
                    console.log('Loading stream URL:', streamUrl);

                    hls.loadSource(streamUrl);
                    hls.attachMedia(videoElement);

                    hls.on(Hls.Events.ERROR, (event, data) => {
                        if (data.fatal) {
                            switch(data.type) {
                                case Hls.ErrorTypes.NETWORK_ERROR:
                                    console.log('Network error, attempting recovery...');
                                    hls?.startLoad();
                                    break;
                                case Hls.ErrorTypes.MEDIA_ERROR:
                                    console.log('Media error, attempting recovery...');
                                    hls?.recoverMediaError();
                                    break;
                                default:
                                    handleError('Fatal streaming error occurred');
                                    break;
                            }
                        }
                    });

                    hls.on(Hls.Events.MANIFEST_PARSED, () => {
                        console.log('Manifest parsed successfully, starting playback');
                        status = 'ready';
                        videoElement.play()
                            .then(() => {
                                console.log('Playback started successfully');
                                resolve();
                            })
                            .catch(e => {
                                console.error('Playback error:', e);
                                reject(new Error(`Playback error: ${e.message}`));
                            });
                    });
                } catch (error) {
                    reject(error);
                }
            };

            tryInitPlayer();
        });
    }

    function startHeartbeat() {
        stopHeartbeat();
        
        heartbeatInterval = setInterval(async () => {
            try {
                const response = await fetch(`${baseUrl}/api/heartbeat`, {
                    method: 'POST'
                });

                if (!response.ok) {
                    throw new Error('Heartbeat failed');
                }
            } catch (error) {
                console.error('Heartbeat error:', error);
                handleError('Connection lost');
            }
        }, HEARTBEAT_INTERVAL);
    }

    function stopHeartbeat() {
        if (heartbeatInterval) {
            clearInterval(heartbeatInterval);
            heartbeatInterval = null;
        }
    }

    function handleError(message: string) {
        console.error('Stream error:', message);
        status = 'error';
        errorMessage = message;
        cleanup();
    }

    function cleanup() {
        if (hls) {
            hls.destroy();
            hls = null;
        }
        
        if (videoElement) {
            videoElement.removeAttribute('src');
            videoElement.load();
        }
        
        stopHeartbeat();
    }

    async function startStream() {
        await initializeStream();
    }

    async function retry() {
        cleanup();
        await initializeStream();
    }

    onDestroy(() => {
        cleanup();
    });
</script>

<div class="relative flex h-96 w-full flex-col items-center justify-center rounded-lg bg-gray-100 lg:w-3/4">
    {#if status === 'idle'}
        <div class="flex flex-col items-center space-y-4 text-gray-600">
            <h2 class="text-2xl font-bold">{text}</h2>
            <button
                on:click={startStream}
                class="flex items-center space-x-2 rounded-full bg-blue-500 p-4 text-white hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                aria-label="Play stream"
            >
                <Play />
            </button>
        </div>
    {:else if status === 'initializing'}
        <div class="flex flex-col items-center space-y-4 text-gray-600">
            <h2 class="text-2xl font-bold">{text}</h2>
            <div class="flex items-center space-x-2">
                <div class="h-4 w-4 animate-spin rounded-full border-2 border-gray-600 border-t-transparent"></div>
                <p>Starting stream...</p>
            </div>
        </div>
    {:else if status === 'loading-player' || status === 'ready'}
        <video
            bind:this={videoElement}
            class="h-full w-full rounded-lg object-contain bg-gray-100"
            controls
            playsinline
            muted
        ></video>
    {:else if status === 'error'}
        <div class="flex flex-col items-center space-y-4 text-gray-600">
            <h2 class="text-2xl font-bold">{text}</h2>
            <p class="text-red-500">{errorMessage}</p>
            <button
                on:click={retry}
                class="rounded bg-blue-500 px-4 py-2 text-white hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
                Retry
            </button>
        </div>
    {/if}
</div>
