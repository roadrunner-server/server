<?php
/**
 * Echoes the value of the environment variable named in the request body, so a
 * test can prove server.env reached the worker process.
 *
 * @var Goridge\RelayInterface $relay
 */

use Spiral\Goridge;
use Spiral\RoadRunner;

$rr = new RoadRunner\Worker($relay);

while ($in = $rr->waitPayload()) {
    try {
        $rr->respond(new RoadRunner\Payload((string)getenv((string)$in->body)));
    } catch (\Throwable $e) {
        $rr->error((string)$e);
    }
}
