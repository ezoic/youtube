package youtube

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTranscript(t *testing.T) {
	video := &Video{ID: "9_MbW9FK1fA"}

	transcript, err := testClient.GetTranscript(video, "en")
	require.NoError(t, err, "get transcript")
	require.Greater(t, len(transcript), 0, "no transcript segments found")

	for i, segment := range transcript {
		index := strconv.Itoa(i)

		require.NotEmpty(t, segment.Text, "text "+index)
		require.NotEmpty(t, segment.Duration, "duration "+index)
		require.NotEmpty(t, segment.OffsetText, "offset "+index)

		if i != 0 {
			require.NotEmpty(t, segment.StartMs, "startMs "+index)
		}
	}

	t.Log(transcript.String())
}

func TestTranscriptOtherLanguage(t *testing.T) {
	video := &Video{ID: "AXwDvYh2-uk"}

	transcript, err := testClient.GetTranscript(video, "id")
	require.NoError(t, err, "get transcript")
	require.Greater(t, len(transcript), 0, "no transcript segments found")

	for i, segment := range transcript {
		index := strconv.Itoa(i)

		require.NotEmpty(t, segment.Text, "text "+index)
		require.NotEmpty(t, segment.Duration, "duration "+index)
		require.NotEmpty(t, segment.OffsetText, "offset "+index)

		if i != 0 {
			require.NotEmpty(t, segment.StartMs, "startMs "+index)
		}
	}

	t.Log(transcript.String())
}

func TestCaptionTrack(t *testing.T) {
	ctx := context.Background()

	video, err := testClient.videoFromID(ctx, "9_MbW9FK1fA")
	require.NoError(t, err, "get video")

	transcript, err := testClient.getCaptionTrackContext(ctx, video, "en")
	require.NoError(t, err, "get caption track")
	require.Greater(t, len(transcript), 0, "no caption track segments found")

	for i, segment := range transcript {
		index := strconv.Itoa(i)

		require.NotEmpty(t, segment.Text, "text "+index)
		require.NotEmpty(t, segment.Duration, "duration "+index)
		require.NotEmpty(t, segment.OffsetText, "offset "+index)

		if i != 0 {
			require.NotEmpty(t, segment.StartMs, "startMs "+index)
		}
	}

	t.Log(transcript.String())
}
