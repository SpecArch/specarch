const express = require('express');
const { Router } = require('express');
const { allow } = require('./guard');

const app = express();
const parts = Router();
const store = [];

/**
 * @param {import('express').Request<{}, {}, import('./types').NewPart>} req
 * @param {import('express').Response} res
 */
function addPart(req, res) {
  store.push({ name: req.body.name });
  res.status(201).end();
}

parts.post('/', allow('parts.write'), addPart);
parts.post('/:partId/moves', allow('parts.write'), (req, res) => {
  const { quantity, location } = req.body;
  res.json({ part: req.params.partId, quantity, location });
});
parts.post('/import', allow('parts.write'), (req, res) => {
  store.push(...req.body);
  res.end();
});

app.use(express.json());
app.use('/parts', parts);

const plugin = process.env.PARTS_PLUGIN;
const extra = require(plugin);
import(`./locales/${process.env.LANG}.js`).then(() => {});
for (const name of ['a', 'b']) {
  module.exports[name] = extra;
}

app.listen(process.env.PORT || 4000);
