using System.IO;
using System.Text;
using BepInEx;
using Rhythm;
using UnityEngine;

namespace UnbeatableRPC
{
    internal static class CoverDump
    {
        private const int MaxKey = 32; // Discord's asset name limit

        internal static string Key(string song)
        {
            if (string.IsNullOrEmpty(song)) return null;
            var sb = new StringBuilder("cover_");
            foreach (char c in song.ToLowerInvariant())
                sb.Append(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ? c : '_');
            return sb.Length > MaxKey ? sb.ToString(0, MaxKey) : sb.ToString();
        }

        internal static bool Dump()
        {
            var index = BeatmapIndex.defaultIndex;
            var names = index?.SongNames;
            if (names == null || names.Length == 0) return false;

            string dir = Path.Combine(Paths.CachePath, "UnbeatableRPC-covers");
            Directory.CreateDirectory(dir);
            var keys = new System.Collections.Generic.SortedSet<string>();
            foreach (string name in names)
            {
                // Story/brawl variants have no CoverArt; covers.go aliases them to their arcade cover.
                if (!index.TryGetSong(name, out var song) || song.CoverArt == null) continue;
                File.WriteAllBytes(Path.Combine(dir, Key(name) + ".png"), Png(song.CoverArt));
                keys.Add(Key(name));
            }
            File.WriteAllLines(Path.Combine(dir, "keys.txt"), keys);
            Plugin.Log.LogInfo($"Dumped {keys.Count} covers to {dir}");
            return true;
        }

        // The texture isn't CPU-readable and may be an atlas, so render it and read back the sprite's rect.
        private static byte[] Png(Sprite sprite)
        {
            var tex = sprite.texture;
            var r = sprite.textureRect;
            var rt = RenderTexture.GetTemporary(tex.width, tex.height, 0, RenderTextureFormat.ARGB32, RenderTextureReadWrite.sRGB);
            var prev = RenderTexture.active;
            Graphics.Blit(tex, rt);
            RenderTexture.active = rt;
            var copy = new Texture2D((int)r.width, (int)r.height, TextureFormat.RGBA32, false);
            copy.ReadPixels(r, 0, 0);
            copy.Apply();
            RenderTexture.active = prev;
            RenderTexture.ReleaseTemporary(rt);
            byte[] png = copy.EncodeToPNG();
            Object.Destroy(copy);
            return png;
        }
    }
}
