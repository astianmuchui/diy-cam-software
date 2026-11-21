
'use strict';

require('dotenv').config();
const mqtt = require('mqtt');

const DEVICE_ID = 'IRK17352YV2026';
const BROKER = `mqtt://${process.env.MQTT_BROKER || 'localhost'}:${process.env.MQTT_PORT || 1883}`;
const TOPIC_READINGS = `sonar/devices/${DEVICE_ID}/readings`;
const TOPIC_HEARTBEAT = `sonar/devices/${DEVICE_ID}/heartbeat`;

const READING_INTERVAL_MS = 5_000;
const HEARTBEAT_INTERVAL_MS = 30_000;

const GPS_CENTER = { lat: -0.0917, lon: 34.7680 };

const rand = (min, max) => Math.random() * (max - min) + min;

const randInt = (min, max) => Math.floor(rand(min, max + 1));

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

const r = (v, n = 2) => parseFloat(v.toFixed(n));

let gpsDrift = { lat: 0, lon: 0 };

function nextGPS() {
  gpsDrift.lat = clamp(gpsDrift.lat + rand(-0.00015, 0.00015), -0.0009, 0.0009);
  gpsDrift.lon = clamp(gpsDrift.lon + rand(-0.00015, 0.00015), -0.0009, 0.0009);
  return {
    lat: r(GPS_CENTER.lat + gpsDrift.lat, 6),
    lon: r(GPS_CENTER.lon + gpsDrift.lon, 6),
    accuracy: r(rand(1.5, 8.0), 1),
  };
}

let depth = rand(10.8, 12.8);

function nextDepth() {
  depth = clamp(depth + rand(-0.35, 0.35), 9.5, 14.5);
  return r(depth, 3);
}

function inferFishType(depthM, amplitude) {
  if (depthM < 20.0 && amplitude > 0.6) return 'tilapia';
  if (depthM < 50.0 && amplitude > 0.5) return 'nile_perch';
  if (depthM >= 50.0 && amplitude > 0.4) return 'catfish';
  return 'unknown';
}

function imuFromTilt(tiltDeg, accelMagG, gyroRange) {
  const tiltRad = (tiltDeg * Math.PI) / 180;
  const lateral = accelMagG * Math.sin(tiltRad);
  const split = rand(0.35, 0.65);
  const signX = Math.random() < 0.5 ? -1 : 1;
  const signY = Math.random() < 0.5 ? -1 : 1;

  return {
    accel_x: r(signX * lateral * split, 3),
    accel_y: r(signY * lateral * (1 - split), 3),
    accel_z: r(accelMagG * Math.cos(tiltRad), 3),
    gyro_x: r(rand(-gyroRange, gyroRange), 2),
    gyro_y: r(rand(-gyroRange, gyroRange), 2),
    gyro_z: r(rand(-gyroRange * 0.7, gyroRange * 0.7), 2),
  };
}

const CASES = [
  {
    name: 'valid-calm',
    weight: 50,
    gateValid: true,
    generate: () => {
      const depthM = nextDepth();
      const tilt = rand(0.5, 8.5);
      return {
        distanceCM: r((depthM * 100) / Math.cos((tilt * Math.PI) / 180), 1),
        imu: imuFromTilt(tilt, rand(0.92, 1.08), 5),
      };
    },
  },
  {
    name: 'valid-motion',
    weight: 20,
    gateValid: true,
    generate: () => {
      const depthM = nextDepth();
      const tilt = rand(10, 22);
      return {
        distanceCM: r((depthM * 100) / Math.cos((tilt * Math.PI) / 180), 1),
        imu: imuFromTilt(tilt, rand(0.88, 1.22), 16),
      };
    },
  },
  {
    name: 'invalid-tilt',
    weight: 10,
    gateValid: false,
    generate: () => {
      const depthM = nextDepth();
      const tilt = rand(28, 42);
      return {
        distanceCM: r((depthM * 100) / Math.cos((tilt * Math.PI) / 180), 1),
        imu: imuFromTilt(tilt, rand(0.95, 1.05), 10),
      };
    },
  },
  {
    name: 'invalid-distance',
    weight: 10,
    gateValid: false,
    generate: () => {
      const tilt = rand(2, 12);
      return {
        distanceCM: r(rand(55, 95), 1),
        imu: imuFromTilt(tilt, rand(0.95, 1.05), 6),
      };
    },
  },
  {
    name: 'invalid-depth-only',
    weight: 5,
    gateValid: false,
    generate: () => {
      const tilt = rand(18, 24.5);
      return {
        distanceCM: r(rand(103, 112), 1),
        imu: imuFromTilt(tilt, rand(0.95, 1.05), 7),
      };
    },
  },
  {
    name: 'invalid-accel',
    weight: 5,
    gateValid: false,
    generate: () => {
      const depthM = nextDepth();
      const tilt = rand(4, 16);
      return {
        distanceCM: r((depthM * 100) / Math.cos((tilt * Math.PI) / 180), 1),
        imu: imuFromTilt(tilt, rand(1.55, 2.1), 7),
      };
    },
  },
];

function pickCase() {
  const total = CASES.reduce((sum, c) => sum + c.weight, 0);
  let n = rand(0, total);
  for (const c of CASES) {
    n -= c.weight;
    if (n <= 0) return c;
  }
  return CASES[0];
}

let battery = 3.75;
let batteryDir = -1;

function nextBattery() {
  battery += batteryDir * rand(0.001, 0.005);
  if (battery <= 3.20) batteryDir = 1;
  if (battery >= 3.80) batteryDir = -1;
  return r(battery, 3);
}

function buildReading() {
  const gps = nextGPS();
  const mode = pickCase();
  const scenario = mode.generate();
  const distCM = scenario.distanceCM;
  const imu = scenario.imu;
  const bat = nextBattery();
  const tiltDeg = Math.atan2(
    Math.sqrt(imu.accel_x ** 2 + imu.accel_y ** 2),
    Math.abs(imu.accel_z),
  ) * 180 / Math.PI;
  const depthM = r((distCM * Math.cos((tiltDeg * Math.PI) / 180)) / 100, 3);

  const fishDetected = mode.gateValid && Math.random() < 0.35;
  const amplitude = fishDetected ? r(rand(0.45, 0.92), 2) : r(rand(0.05, 0.28), 2);
  const fishCount = fishDetected ? randInt(1, 12) : 0;
  const fishType = fishDetected ? inferFishType(depthM, amplitude) : '';

  return {
    device_id: DEVICE_ID,
    timestamp: Math.floor(Date.now() / 1000),
    gps: {
      lat: gps.lat,
      lon: gps.lon,
      accuracy: gps.accuracy,
    },
    ultrasonic: {
      distance_cm: distCM,
      valid: true,
    },
    mpu6050: {
      accel_x: imu.accel_x,
      accel_y: imu.accel_y,
      accel_z: imu.accel_z,
      gyro_x: imu.gyro_x,
      gyro_y: imu.gyro_y,
      gyro_z: imu.gyro_z,
    },
    sonar: {
      echo_amplitude: amplitude,
      fish_detected: fishDetected,
      fish_count: fishCount,
      fish_type: fishType,
    },
    system: {
      battery_voltage: bat,
      signal_strength: randInt(-95, -55),
      transmission_type: 'mqtt',
    },
    _meta: {
      mode: mode.name,
      expected_gate_valid: mode.gateValid,
      derived_tilt_deg: r(tiltDeg, 2),
      derived_depth_m: depthM,
    },
  };
}

function buildHeartbeat() {
  return { device_id: DEVICE_ID, status: 'online' };
}

const clientOpts = {
  clientId: `simulator-${DEVICE_ID}-${Date.now()}`,
  clean: true,
  reconnectPeriod: 3000,
};

if (process.env.MQTT_USER) {
  clientOpts.username = process.env.MQTT_USER;
  clientOpts.password = process.env.MQTT_PASSWORD || '';
}

console.log(`[simulator] connecting to ${BROKER} as ${DEVICE_ID}…`);
const client = mqtt.connect(BROKER, clientOpts);

client.on('connect', () => {
  console.log(`[simulator] connected ✓`);

  publishHeartbeat();

  setInterval(publishReading, READING_INTERVAL_MS);

  setInterval(publishHeartbeat, HEARTBEAT_INTERVAL_MS);
});

client.on('reconnect', () => console.log('[simulator] reconnecting…'));
client.on('error', (err) => console.error('[simulator] error:', err.message));
client.on('offline', () => console.log('[simulator] offline'));

function publishReading() {
  const payload = buildReading();
  client.publish(TOPIC_READINGS, JSON.stringify(payload), { qos: 1 }, (err) => {
    if (err) { console.error('[simulator] publish reading error:', err.message); return; }
    const tag = payload.sonar.fish_detected
      ? `🐟 FISH (${payload.sonar.fish_count}x ${payload.sonar.fish_type}, amp=${payload.sonar.echo_amplitude})`
      : '  no fish';
    console.log(
      `[reading] mode=${payload._meta.mode} valid=${payload._meta.expected_gate_valid} ` +
      `tilt≈${payload._meta.derived_tilt_deg}° depth≈${payload._meta.derived_depth_m}m ` +
      `dist=${payload.ultrasonic.distance_cm}cm  ` +
      `gps=${payload.gps.lat},${payload.gps.lon}  ` +
      `bat=${payload.system.battery_voltage}V  ${tag}`
    );
  });
}

function publishHeartbeat() {
  const payload = buildHeartbeat();
  client.publish(TOPIC_HEARTBEAT, JSON.stringify(payload), { qos: 1 }, (err) => {
    if (err) { console.error('[simulator] publish heartbeat error:', err.message); return; }
    console.log(`[heartbeat] ${DEVICE_ID} online`);
  });
}
