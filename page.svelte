<script lang="ts">
    import StreamPlayer from '$lib/StreamPlayer.svelte';
</script>

<div class="container mx-auto px-4 my-4">
	<div class="flex flex-col gap-4 lg:flex-row">
		<Camera text="Camera 1" baseUrl="https://stream1.test.example.com" />
		<Camera text="Camera 2" baseUrl="https://stream2.test.example.com"/>
	</div>
</div>
