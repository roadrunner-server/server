<?php
/**
 * @var Goridge\RelayInterface $relay
 */

use Spiral\Goridge;
use Spiral\RoadRunner;

ini_set('display_errors', 'stderr');
require __DIR__ . "/vendor/autoload.php";

$relay = Goridge\Relay::create(getenv('RR_RELAY'));
$rr = new RoadRunner\Worker($relay);

while ($in = $rr->waitPayload()) {
    try {
        $rr->respond(new RoadRunner\Payload((string)$in->body));
    } catch (\Throwable $e) {
        $rr->error((string)$e);
    }
}
