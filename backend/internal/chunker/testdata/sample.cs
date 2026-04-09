using System;
using System.Collections.Generic;

namespace CodeTextor.Samples
{
    public interface ICalculator
    {
        int Sum(int a, int b);
    }

    public class Calculator : ICalculator
    {
        public int Add(int a, int b)
        {
            return a + b;
        }

        public static void Main(string[] args)
        {
            var calc = new Calculator();
            Console.WriteLine(calc.Add(1, 2));
            /* HACK: testing comments */
        }
    }
}
