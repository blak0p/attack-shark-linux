# Rapid macro clicks can be filtered after evdev

A macro's transport confirmation does not prove that applications receive every
click. In one X6 diagnostic session, evdev reported four left-button click pairs,
while libinput and Firefox each reported one pair.

## Observed boundary

| Layer | Observed result |
|---|---|
| Kernel input / evdev | Four `BTN_LEFT` press/release pairs over about 96 ms. |
| libinput 1.32.0 | One press/release pair lasting about 95 ms. |
| Firefox raw mouse listeners | One press/release pair, consistent with the libinput output. |

This localizes the observed reduction to the input processing between evdev and
libinput output; it does not establish a Firefox-only defect. libinput documents
button debouncing that can suppress rapid transitions and asks for reports when
burst-producing devices are incorrectly filtered:
[libinput button debouncing](https://wayland.freedesktop.org/libinput/doc/latest/button-debouncing.html).
The exact active filter mode was not verified, and no matching upstream issue or
accepted upstream bug is claimed here.

## What this does not prove

The diagnostic USB capture used a local double-upload experiment. That experiment
is not shipped and is not a production fix. Its observations do not prove normal
single-upload firmware behavior, a requirement to duplicate uploads, or correct
playback in every application.

Continuous toggle-stop, while-held release-to-stop, all repeat counts, and device
persistence have not been exhaustively verified on hardware. Offline tests verify
encoding, validation, staging and upload framing, not physical playback. Do not
change global input filtering or introduce unverified timing bytes to work around
this result.

## Reporting a similar result

Compare the affected mouse's evdev events with libinput output before attributing
the problem to an application. Capture only that mouse, not unrelated keyboard
input. Include the libinput version, playback mode and event counts in a report;
do not publish proprietary Windows-app captures or personal host paths.
