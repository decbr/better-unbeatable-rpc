using System.Globalization;
using System.Text;

namespace UnbeatableRPC
{
    internal class StateMessage
    {
        public const int SchemaVersion = 1;

        public string State;
        public string Song;
        public string Artist;
        public string Charter;
        public string Difficulty;
        public int? Level;
        public string Chapter;
        public int? HighScore;
        public float? Accuracy;
        public string Rank;
        public float? Stars;
        public bool? Paused;
        public long? StartedAt;

        public string ToJson()
        {
            var sb = new StringBuilder(256);
            sb.Append("{\"v\":").Append(SchemaVersion);
            Add(sb, "state", State);
            Add(sb, "song", Song);
            Add(sb, "artist", Artist);
            Add(sb, "charter", Charter);
            Add(sb, "difficulty", Difficulty);
            if (Level.HasValue) sb.Append(",\"level\":").Append(Level.Value);
            Add(sb, "chapter", Chapter);
            if (HighScore.HasValue) sb.Append(",\"highScore\":").Append(HighScore.Value);
            if (Accuracy.HasValue) sb.Append(",\"accuracy\":").Append(Accuracy.Value.ToString("0.#####", CultureInfo.InvariantCulture));
            Add(sb, "rank", Rank);
            if (Stars.HasValue) sb.Append(",\"stars\":").Append(Stars.Value.ToString("0.###", CultureInfo.InvariantCulture));
            if (Paused.HasValue) sb.Append(",\"paused\":").Append(Paused.Value ? "true" : "false");
            if (StartedAt.HasValue) sb.Append(",\"startedAt\":").Append(StartedAt.Value);
            return sb.Append('}').ToString();
        }

        private static void Add(StringBuilder sb, string key, string value)
        {
            if (string.IsNullOrEmpty(value)) return;
            sb.Append(",\"").Append(key).Append("\":\"");
            foreach (char c in value)
            {
                switch (c)
                {
                    case '"': sb.Append("\\\""); break;
                    case '\\': sb.Append("\\\\"); break;
                    case '\n': sb.Append("\\n"); break;
                    case '\r': sb.Append("\\r"); break;
                    case '\t': sb.Append("\\t"); break;
                    default:
                        if (c < 0x20) sb.Append("\\u").Append(((int)c).ToString("x4"));
                        else sb.Append(c);
                        break;
                }
            }
            sb.Append('"');
        }
    }
}
