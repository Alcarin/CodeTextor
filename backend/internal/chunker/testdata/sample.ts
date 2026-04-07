import { EventEmitter } from 'events';
import * as path from 'path';

// TODO: add generics support

/**
 * Represents a named entity.
 */
interface Nameable {
    getName(): string;
}

/**
 * A user in the system.
 */
class User implements Nameable {
    private _email: string;

    constructor(public name: string, email: string) {
        this._email = email;
    }

    public getName(): string {
        return this.name;
    }

    private _validate(): boolean {
        return this._email.includes('@');
    }
}

/**
 * Add two numbers.
 */
function add(a: number, b: number): number {
    return a + b;
}

const multiply = (a: number, b: number): number => a * b;

// FIXME: error handling
const result = add(1, 2);
const user = new User("Alice", "alice@example.com");
console.log(user.getName());

watch(result, (value) => {
    console.log(value);
});
