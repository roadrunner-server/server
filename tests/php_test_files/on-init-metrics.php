<?php

declare(strict_types=1);

// deprecation notices on stdout would corrupt nothing here, but keep the
// on_init output clean so a failure is readable in the server log
ini_set('display_errors', 'stderr');

require __DIR__ . '/vendor/autoload.php';

use Spiral\Goridge\RPC\RPC;
use Spiral\RoadRunner\Metrics\Collector;
use Spiral\RoadRunner\Metrics\Metrics;

$metrics = new Metrics(RPC::create('tcp://127.0.0.1:6001'));

$metrics->declare(
    'test',
    Collector::counter()
        ->withNamespace('foo')
        ->withSubsystem('bar')
        ->withHelp('test counter declared from on_init'),
);

$metrics->add('test', 1);
