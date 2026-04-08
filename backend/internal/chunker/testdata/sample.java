package com.example.codetextor;

import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.util.*;

/**
 * Service class for testing purpose.
 */
public class CalculatorService implements IService {
    
    private static final int DEFAULT_VALUE = 42;
    private List<String> history = new ArrayList<>();

    public CalculatorService() {
        // Constructor
        System.out.println("Service initialized");
    }

    /*
     * Adds two numbers.
     * TODO: Add support for double
     */
    public int add(int a, int b) {
        return a + b;
    }

    /**
     * Executes a complex operation.
     * FIXME: Refactor this method
     */
    @Override
    public void execute() {
        System.out.println("Executing...");
        // HACK: Temporary workaround
        int result = add(DEFAULT_VALUE, 10);
        System.out.println("Result: " + result);
    }

    public enum OperationType {
        ADD, SUBTRACT, MULTIPLY
    }

    private interface IService {
        void execute();
    }
}
