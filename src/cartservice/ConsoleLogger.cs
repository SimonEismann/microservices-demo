using zipkin4net;
using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Threading.Tasks;

namespace cartservice
{
    class ConsoleLogger : ILogger
    {
        public void LogError(string message)
        {
            Console.Error.WriteLine("ZIPKIN ERROR: " + message);
        }

        public void LogInformation(string message)
        {
            Console.WriteLine("ZIPKIN LOG: " + message);
        }

        public void LogWarning(string message)
        {
            Console.ForegroundColor = ConsoleColor.Yellow;
            Console.WriteLine("ZIPKIN WARNING: " + message);
            Console.ResetColor();
        }
    }
}