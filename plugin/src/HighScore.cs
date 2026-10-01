namespace UnbeatableRPC
{
    internal static class HighScore
    {
        internal class Best
        {
            public int Score;
            public float Accuracy;
            public string Grade;
        }

        internal static string BoardKey(string beatmapPath) =>
            beatmapPath + HighScoreList.GetModifiersLeaderboard(StorableBeatmapOptions.GetModifierMask());

        internal static Best Lookup(string key)
        {
            var scores = FileStorage.highscores?._highScores;
            if (key == null || scores == null || !scores.TryGetValue(key, out var item) || item.score <= 0)
                return null;

            bool noMiss = item.isNoMiss ?? (item._notes == null
                                            || !item._notes.TryGetValue("Miss", out int misses)
                                            || misses == 0);
            return new Best
            {
                Score = item.score,
                Accuracy = item.accuracy,
                Grade = HighScoreScreen.GetLetterGradeArcade(item.accuracy, noMiss, item.cleared),
            };
        }
    }
}
