using System;
using Arcade.Utils;
using HarmonyLib;
using Rhythm;
using UnityEngine;

namespace UnbeatableRPC.Patches
{
    [HarmonyPatch(typeof(DiscordComponent), "Awake")]
    internal static class DiscordComponentAwake
    {
        private static bool logged;

        private static bool Prefix(DiscordComponent __instance)
        {
            try
            {
                if (!Plugin.DisableGameDiscord.Value) return true;
                UnityEngine.Object.DontDestroyOnLoad(__instance.gameObject);
                if (!logged)
                {
                    Plugin.Log.LogInfo("Game's Discord integration disabled");
                    logged = true;
                }
                return false;
            }
            catch (Exception e)
            {
                Plugin.LogOnce(nameof(DiscordComponentAwake), e);
                return true;
            }
        }
    }

    [HarmonyPatch(typeof(RhythmController), nameof(RhythmController.InitializeAndPlay))]
    internal static class RhythmControllerInitializeAndPlay
    {
        private static void Postfix(RhythmController __instance)
        {
            try
            {
                GameTracker.OnSongStarted(__instance);
            }
            catch (Exception e)
            {
                Plugin.LogOnce(nameof(RhythmControllerInitializeAndPlay), e);
            }
        }
    }

    [HarmonyPatch(typeof(RhythmController), "OnDestroy")]
    internal static class RhythmControllerOnDestroy
    {
        private static void Postfix(RhythmController __instance)
        {
            try
            {
                GameTracker.OnSongEnded(__instance);
            }
            catch (Exception e)
            {
                Plugin.LogOnce(nameof(RhythmControllerOnDestroy), e);
            }
        }
    }

    [HarmonyPatch(typeof(HighScoreScreen), "Start")]
    internal static class HighScoreScreenStart
    {
        private static void Postfix()
        {
            try
            {
                GameTracker.OnResults();
            }
            catch (Exception e)
            {
                Plugin.LogOnce(nameof(HighScoreScreenStart), e);
            }
        }
    }

    // Never call GetPlayerRank ourselves: it writes stats and can complete a challenge.
    [HarmonyPatch(typeof(PlayerStatsHelper), nameof(PlayerStatsHelper.GetPlayerRank))]
    internal static class PlayerStatsHelperGetPlayerRank
    {
        private static void Postfix(float __result)
        {
            try
            {
                GameTracker.OnPlayerRank(__result);
            }
            catch (Exception e)
            {
                Plugin.LogOnce(nameof(PlayerStatsHelperGetPlayerRank), e);
            }
        }
    }
}
