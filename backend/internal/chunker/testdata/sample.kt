package com.example.project

import kotlinx.coroutines.*
import java.util.UUID

/**
 * A sample class in Kotlin.
 */
class Calculator(val name: String) {
    
    // TODO: support decimals
    fun add(a: Int, b: Int): Int {
        return a + b
    }

    private fun hiddenMethod() {
        println("Hidden")
    }
}

object Singleton {
    val version = "1.0.0"
    
    fun info() = println("Kotlin $version")
}

interface Shape {
    fun area(): Double
}

fun topLevelFunction() {
    println("Global scope")
}

val globalVar = 42
