/// A sample structure
pub struct Calculator {
    value: i32,
}

impl Calculator {
    /// Creates a new calculator
    pub fn new() -> Self {
        Calculator { value: 0 }
    }

    /// Adds a value
    pub fn add(&mut self, n: i32) {
        // TODO: handle overflow
        self.value += n;
    }

    pub fn get_value(&self) -> i32 {
        self.value
    }
}

pub trait Display {
    fn show(&self);
}

pub enum State {
    Running,
    Stopped,
}

mod internal {
    fn secret() {}
}

const PI: f64 = 3.14159;

use std::collections::HashMap;

fn main() {
    let mut c = Calculator::new();
    c.add(10);
    println!("Value: {}", c.get_value());
}
