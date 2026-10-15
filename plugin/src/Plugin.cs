using System;
using System.Collections.Generic;
using BepInEx;
using BepInEx.Configuration;
using BepInEx.Logging;
using HarmonyLib;
using UnityEngine;

namespace UnbeatableRPC
{
    [BepInPlugin(Guid, Name, Version)]
    public class Plugin : BaseUnityPlugin
    {
        public const string Guid = "xyz.diaduck.unbeatable-rpc";
        public const string Name = "UnbeatableRPC";
        public const string Version = "0.3.2";

        internal static ManualLogSource Log;
        internal static ConfigEntry<int> UdpPort;
        internal static ConfigEntry<bool> DisableGameDiscord;
        internal static ConfigEntry<bool> DebugLogging;
        internal static ConfigEntry<float> LastKnownStars;
        internal static long LaunchedAt;

        private void Awake()
        {
            Log = Logger;
            LaunchedAt = GameLaunchTime();

            UdpPort = Config.Bind("General", "UdpPort", 47820,
                "UDP port on 127.0.0.1 that the unbeatable-rpc daemon listens on.");
            DisableGameDiscord = Config.Bind("General", "DisableGameDiscord", true,
                "Stop the game's built-in Discord Game SDK integration from starting.");
            DebugLogging = Config.Bind("General", "DebugLogging", false,
                "Log scene changes and every state message sent.");
            LastKnownStars = Config.Bind("State", "LastKnownStars", -1f,
                "Last player rank (star rating) seen, so it can be shown before the game recomputes it. Managed by the plugin; -1 means unknown.");

            ApplyPatches();

            var host = new GameObject("UnbeatableRPC");
            host.hideFlags = HideFlags.HideAndDontSave;
            DontDestroyOnLoad(host);
            host.AddComponent<GameTracker>();

            Log.LogInfo($"{Name} {Version} loaded; sending to 127.0.0.1:{UdpPort.Value}");
        }

        // Unix seconds the game started. The process start time if Wine reports a
        // sane one, else now: BepInEx loads plugins moments after launch anyway.
        private static long GameLaunchTime()
        {
            var now = DateTimeOffset.UtcNow;
            try
            {
                var start = new DateTimeOffset(System.Diagnostics.Process.GetCurrentProcess().StartTime.ToUniversalTime());
                if (start <= now && now - start < TimeSpan.FromMinutes(10)) return start.ToUnixTimeSeconds();
            }
            catch (Exception e)
            {
                Debug($"No process start time: {e.Message}");
            }
            return now.ToUnixTimeSeconds();
        }

        private static void ApplyPatches()
        {
            var harmony = new Harmony(Guid);
            foreach (var type in new[]
                     {
                         typeof(Patches.DiscordComponentAwake),
                         typeof(Patches.RhythmControllerInitializeAndPlay),
                         typeof(Patches.RhythmControllerOnDestroy),
                         typeof(Patches.HighScoreScreenStart),
                         typeof(Patches.PlayerStatsHelperGetPlayerRank),
                     })
            {
                try
                {
                    harmony.CreateClassProcessor(type).Patch();
                    if (DebugLogging.Value) Log.LogInfo($"Patched {type.Name}");
                }
                catch (Exception e)
                {
                    Log.LogError($"Failed to apply {type.Name}: {e}");
                }
            }
        }

        private static readonly HashSet<string> LoggedErrors = new HashSet<string>();

        internal static void LogOnce(string where, Exception e)
        {
            if (LoggedErrors.Add(where)) Log?.LogError($"{where}: {e}");
        }

        internal static void Debug(string message)
        {
            if (DebugLogging != null && DebugLogging.Value) Log?.LogInfo(message);
        }
    }
}
