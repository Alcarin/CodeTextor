import { readFile } from 'fs';
import express from 'express';

// TODO: implement caching

/**
 * Application controller.
 */
class AppController {
    constructor() {
        this.name = 'app';
    }

    start() {
        console.log('Starting');
    }

    _internalSetup() {
        // HACK: workaround for init race
    }
}

function handleRequest(req, res) {
    res.send('OK');
}

const processData = (data) => {
    return data.map(item => item.value);
};

const app = new AppController();
app.start();
readFile('/tmp/data.txt', console.log);
