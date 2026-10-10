using System;
using System.Collections.Generic;
using NarrativeGraph.ChapterIndex;
using Rhythm;
using UnityEngine;
using UnityEngine.SceneManagement;

namespace UnbeatableRPC
{
    internal class GameTracker : MonoBehaviour
    {
        private const float PollInterval = 0.5f;
        private const float HeartbeatInterval = 10f;
        private const string ResultsScene = "ScoreScreenArcadeMode";

        private static readonly HashSet<string> MenuScenes = new HashSet<string>
        {
            "C2_MainMenu", "ArcadeModeMenu", ResultsScene, "BootUp", "C4_LogoScreen",
            "Epilepsy Warning", "OffsetWizard", "OffsetWizardResults", "ShutDown",
        };

        private class SongFacts
        {
            public bool Arcade;
            public string Song;
            public string Artist;
            public string Charter;
            public string Difficulty;
            public int Level;
            public string BoardKey;
            public HighScore.Best Best;
        }

        private static RhythmController currentController;
        private static SongFacts currentSong;
        private static SongFacts lastArcadeSong;
        private static bool inResults;
        private static float? stars;
        private static long startedAt;

        private UdpSender sender;
        private float nextPoll;
        private float lastSendTime = float.NegativeInfinity;
        private string lastJson;
        private string lastScene;
        private string currentChapter;

        private void Start()
        {
            sender = new UdpSender(Plugin.UdpPort.Value);
            if (Plugin.LastKnownStars.Value >= 0f) stars = Plugin.LastKnownStars.Value;
        }

        internal static void OnSongStarted(RhythmController controller)
        {
            var meta = controller.beatmap != null ? controller.beatmap.metadata : null;
            var progression = JeffBezosController.rhythmProgression;
            var facts = new SongFacts
            {
                Arcade = JeffBezosController.rhythmGameType == RhythmGameType.ArcadeMode
                         && progression is ArcadeProgression,
                Song = NonEmpty(meta?.title) ?? NonEmpty(progression?.GetSongName()),
                Artist = NonEmpty(meta?.artist),
                Charter = NonEmpty(meta?.creator),
                Difficulty = NonEmpty(meta?.GetDifficulty(progression?.GetDifficulty()))
                             ?? NonEmpty(progression?.GetDifficulty()),
                Level = meta != null ? meta.tagData.Level : 0,
            };
            if (facts.Arcade)
            {
                facts.BoardKey = HighScore.BoardKey(progression.GetBeatmapPath());
                facts.Best = HighScore.Lookup(facts.BoardKey);
                lastArcadeSong = facts;
            }

            currentController = controller;
            currentSong = facts;
            inResults = false;
            startedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
            Plugin.Debug($"Song started: {facts.Song} [{facts.Difficulty}] arcade={facts.Arcade} board={facts.BoardKey} pb={facts.Best?.Score}");
        }

        internal static void OnSongEnded(RhythmController controller)
        {
            if (!ReferenceEquals(controller, currentController)) return;
            currentController = null;
            currentSong = null;
            Plugin.Debug("Song ended");
        }

        internal static void OnResults()
        {
            if (lastArcadeSong == null || !(JeffBezosController.rhythmProgression is ArcadeProgression)) return;
            lastArcadeSong.Best = HighScore.Lookup(lastArcadeSong.BoardKey);
            inResults = true;
            Plugin.Debug($"Results: pb={lastArcadeSong.Best?.Score}");
        }

        internal static void OnPlayerRank(float rank)
        {
            if (stars.HasValue && Math.Abs(stars.Value - rank) < 0.0005f) return;
            stars = rank;
            Plugin.LastKnownStars.Value = rank;
            Plugin.Debug($"Player rank: {rank:0.000}");
        }

        private void Update()
        {
            if (Time.unscaledTime < nextPoll) return;
            nextPoll = Time.unscaledTime + PollInterval;

            string json;
            try
            {
                json = BuildMessage().ToJson();
            }
            catch (Exception e)
            {
                Plugin.LogOnce("GameTracker.BuildMessage", e);
                return;
            }

            bool changed = json != lastJson;
            if (!changed && Time.unscaledTime - lastSendTime < HeartbeatInterval) return;

            sender.Send(json);
            lastJson = json;
            lastSendTime = Time.unscaledTime;
            if (changed) Plugin.Debug($"Sent {json}");
        }

        private void OnApplicationQuit()
        {
            sender?.Send(new StateMessage { State = "closed" }.ToJson());
            sender?.Dispose();
        }

        private StateMessage BuildMessage()
        {
            string scene = SceneManager.GetActiveScene().name;
            if (scene != lastScene)
            {
                lastScene = scene;
                if (!MenuScenes.Contains(scene))
                {
                    string chapter = ChapterFor(scene);
                    if (chapter != null) currentChapter = chapter;
                }
                Plugin.Debug($"Scene: {scene} (chapter: {currentChapter ?? "?"})");
            }
            if (scene != ResultsScene) inResults = false;

            var msg = new StateMessage { Stars = stars };

            if (currentSong != null && currentController != null)
            {
                var s = currentSong;
                msg.State = s.Arcade ? "arcade" : "story";
                msg.Song = s.Song;
                msg.Artist = s.Artist;
                msg.Charter = s.Charter;
                msg.Difficulty = s.Difficulty;
                msg.Level = s.Level > 0 ? s.Level : (int?)null;
                if (!s.Arcade) msg.Chapter = currentChapter;
                ApplyBest(msg, s.Best);

                // inPauseMenu, not isPaused: isPaused is also true on loading screens (e.g. after the last note).
                bool paused = JeffBezosController.inPauseMenu || currentController.song.isPausedWait;
                msg.Paused = paused;
                if (!paused)
                {
                    ResyncStart();
                    msg.StartedAt = startedAt;
                }
                return msg;
            }

            if (inResults && lastArcadeSong != null)
            {
                msg.State = "results";
                msg.Song = lastArcadeSong.Song;
                msg.Artist = lastArcadeSong.Artist;
                msg.Charter = lastArcadeSong.Charter;
                msg.Difficulty = lastArcadeSong.Difficulty;
                msg.Level = lastArcadeSong.Level > 0 ? lastArcadeSong.Level : (int?)null;
                ApplyBest(msg, lastArcadeSong.Best);
                return msg;
            }

            if (!MenuScenes.Contains(scene) && ChapterFor(scene) != null)
            {
                msg.State = "story";
                msg.Chapter = currentChapter;
                return msg;
            }

            msg.State = "idle";
            return msg;
        }

        private static void ResyncStart()
        {
            float positionMs = Mathf.Max(0f, JeffBezosController.prevSongTime);
            float speed = Mathf.Abs(currentController.timeScale);
            if (speed < 0.01f) return;
            long expected = DateTimeOffset.UtcNow.ToUnixTimeSeconds() - (long)(positionMs / 1000f / speed);
            if (Math.Abs(expected - startedAt) > 2) startedAt = expected;
        }

        private static void ApplyBest(StateMessage msg, HighScore.Best best)
        {
            if (best == null) return;
            msg.HighScore = best.Score;
            msg.Accuracy = best.Accuracy;
            msg.Rank = best.Grade;
        }

        private static string ChapterFor(string scene)
        {
            try
            {
                var index = ChapterSceneIndex.Instance;
                if (index?.SceneChapters != null && index.SceneChapters.TryGetValue(scene, out var chapter))
                    return chapter;
            }
            catch (Exception e)
            {
                Plugin.LogOnce("GameTracker.ChapterFor", e);
            }
            return null;
        }

        private static string NonEmpty(string s) => string.IsNullOrWhiteSpace(s) ? null : s;
    }
}
