using System;
using System.Net.Sockets;
using System.Text;

namespace UnbeatableRPC
{
    internal sealed class UdpSender : IDisposable
    {
        private readonly UdpClient client;
        private readonly int port;

        public UdpSender(int port)
        {
            this.port = port;
            try
            {
                client = new UdpClient();
            }
            catch (Exception e)
            {
                Plugin.LogOnce("UdpSender.ctor", e);
            }
        }

        public void Send(string json)
        {
            if (client == null) return;
            try
            {
                byte[] data = Encoding.UTF8.GetBytes(json);
                client.Send(data, data.Length, "127.0.0.1", port);
            }
            catch (Exception e)
            {
                Plugin.LogOnce("UdpSender.Send", e);
            }
        }

        public void Dispose()
        {
            try { client?.Dispose(); } catch { }
        }
    }
}
