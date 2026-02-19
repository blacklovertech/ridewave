package db

import (
	"context"
	"log"
)

// Migrate creates all tables if they don't exist, adds columns, indexes, and seeds default data.
// Safe to run multiple times — all operations are idempotent (IF NOT EXISTS / ON CONFLICT).
func Migrate() {
	sql := `
	CREATE EXTENSION IF NOT EXISTS pgcrypto;

	-- ═══════════════════════════════════════════
	-- USERS TABLE
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS "user" (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		name TEXT,
		phone_number TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE,
		"notificationToken" TEXT,
		ratings DOUBLE PRECISION NOT NULL DEFAULT 0,
		"totalRides" DOUBLE PRECISION NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'active',
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE "user" ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

	-- ═══════════════════════════════════════════
	-- DRIVERS TABLE
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS driver (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		name TEXT NOT NULL,
		country TEXT NOT NULL,
		phone_number TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		vehicle_type TEXT NOT NULL,
		registration_number TEXT UNIQUE NOT NULL,
		registration_date TEXT NOT NULL,
		driving_license TEXT NOT NULL,
		vehicle_color TEXT,
		rate TEXT NOT NULL,
		"notificationToken" TEXT,
		ratings DOUBLE PRECISION NOT NULL DEFAULT 0,
		"totalEarning" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"totalRides" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"totalDistance" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"pendingRides" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"cancelRides" DOUBLE PRECISION NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'inactive',
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- Driver profile extensions
	ALTER TABLE driver ADD COLUMN IF NOT EXISTS "profileImage" TEXT;
	ALTER TABLE driver ADD COLUMN IF NOT EXISTS "rcBook" TEXT;
	ALTER TABLE driver ADD COLUMN IF NOT EXISTS "isOnline" BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE driver ADD COLUMN IF NOT EXISTS "upi_id" TEXT;

	-- ═══════════════════════════════════════════
	-- RIDES TABLE — full ride lifecycle
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS rides (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		"userId" TEXT NOT NULL REFERENCES "user"(id),
		"driverId" TEXT REFERENCES driver(id),
		charge DOUBLE PRECISION NOT NULL,
		"currentLocationName" TEXT NOT NULL,
		"destinationLocationName" TEXT NOT NULL,
		distance TEXT NOT NULL,
		polyline TEXT,
		"estimatedDuration" INTEGER,
		"estimatedDistance" INTEGER,
		"vehicleType" TEXT,
		status TEXT NOT NULL DEFAULT 'Requested',
		rating DOUBLE PRECISION,
		"paymentMode" TEXT,
		"paymentStatus" TEXT DEFAULT 'Pending',
		"cancelReason" TEXT,
		otp TEXT,
		"originLat" DOUBLE PRECISION,
		"originLng" DOUBLE PRECISION,
		"destinationLat" DOUBLE PRECISION,
		"destinationLng" DOUBLE PRECISION,
		"acceptedAt" TIMESTAMPTZ,
		"startedAt" TIMESTAMPTZ,
		"completedAt" TIMESTAMPTZ,
		"cancelledAt" TIMESTAMPTZ,
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- Safe column additions for existing databases
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "originLat" DOUBLE PRECISION;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "originLng" DOUBLE PRECISION;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "destinationLat" DOUBLE PRECISION;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "destinationLng" DOUBLE PRECISION;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS polyline TEXT;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "estimatedDuration" INTEGER;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "estimatedDistance" INTEGER;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "vehicleType" TEXT;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "paymentMode" TEXT;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "paymentStatus" TEXT DEFAULT 'Pending';
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "cancelReason" TEXT;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS otp TEXT;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "acceptedAt" TIMESTAMPTZ;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "startedAt" TIMESTAMPTZ;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "completedAt" TIMESTAMPTZ;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "cancelledAt" TIMESTAMPTZ;
	ALTER TABLE rides ALTER COLUMN "driverId" DROP NOT NULL;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS tips DOUBLE PRECISION DEFAULT 0;
	ALTER TABLE rides ADD COLUMN IF NOT EXISTS "routeId" TEXT;

	-- ═══════════════════════════════════════════
	-- DRIVER LIVE LOCATION TABLE
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS driver_location (
		"driverId" TEXT PRIMARY KEY REFERENCES driver(id),
		lat DOUBLE PRECISION NOT NULL,
		lng DOUBLE PRECISION NOT NULL,
		heading DOUBLE PRECISION,
		speed DOUBLE PRECISION,
		"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	ALTER TABLE driver_location ADD COLUMN IF NOT EXISTS speed DOUBLE PRECISION;

	-- ═══════════════════════════════════════════
	-- PAYMENTS TABLE
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS payments (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		"rideId" TEXT NOT NULL REFERENCES rides(id),
		amount DOUBLE PRECISION NOT NULL,
		mode TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- ═══════════════════════════════════════════
	-- VEHICLE TYPES TABLE (DB-driven fare config)
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS vehicle_types (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		name TEXT UNIQUE NOT NULL,
		"baseFare" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"perKmRate" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"perMinRate" DOUBLE PRECISION NOT NULL DEFAULT 0,
		icon TEXT,
		"isActive" BOOLEAN NOT NULL DEFAULT TRUE,
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- Seed default vehicle types (only inserts if not already present)
	INSERT INTO vehicle_types (id, name, "baseFare", "perKmRate", "perMinRate", icon) VALUES
		(gen_random_uuid()::text, 'Auto', 40.0, 10.0, 1.5, 'auto'),
		(gen_random_uuid()::text, 'Scooter', 25.0, 7.0, 0.8, 'scooter'),
		(gen_random_uuid()::text, 'Motorcycle', 30.0, 8.0, 1.0, 'motorcycle'),
		(gen_random_uuid()::text, 'Car', 50.0, 12.0, 2.0, 'car'),
		(gen_random_uuid()::text, 'Sedan', 65.0, 15.0, 2.5, 'sedan'),
		(gen_random_uuid()::text, 'SUV', 80.0, 18.0, 3.0, 'suv')
	ON CONFLICT (name) DO NOTHING;

	-- ═══════════════════════════════════════════
	-- SOS ALERTS TABLE — safety audit trail
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS sos_alerts (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		"rideId" TEXT REFERENCES rides(id),
		"userId" TEXT NOT NULL REFERENCES "user"(id),
		lat DOUBLE PRECISION,
		lng DOUBLE PRECISION,
		status TEXT NOT NULL DEFAULT 'active',
		"resolvedAt" TIMESTAMPTZ,
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- ═══════════════════════════════════════════
	-- PROMO CODES TABLE — discount management
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS promo_codes (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		code TEXT UNIQUE NOT NULL,
		"discountType" TEXT NOT NULL DEFAULT 'percentage',
		"discountValue" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"maxDiscount" DOUBLE PRECISION,
		"minRideAmount" DOUBLE PRECISION NOT NULL DEFAULT 0,
		"usageLimit" INTEGER NOT NULL DEFAULT 1,
		"usedCount" INTEGER NOT NULL DEFAULT 0,
		"expiresAt" TIMESTAMPTZ,
		"isActive" BOOLEAN NOT NULL DEFAULT TRUE,
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- ═══════════════════════════════════════════
	-- FIX: Rename cratedAt → createdAt (safe migration)
	-- ═══════════════════════════════════════════
	DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='user' AND column_name='cratedAt') THEN
			ALTER TABLE "user" RENAME COLUMN "cratedAt" TO "createdAt";
		END IF;
	END $$;

	DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='rides' AND column_name='cratedAt') THEN
			ALTER TABLE rides RENAME COLUMN "cratedAt" TO "createdAt";
		END IF;
	END $$;

	-- ═══════════════════════════════════════════
	-- INDEXES — optimized for all API queries
	-- ═══════════════════════════════════════════
	-- Core lookups
	CREATE INDEX IF NOT EXISTS idx_rides_userid ON rides("userId");
	CREATE INDEX IF NOT EXISTS idx_rides_driverid ON rides("driverId");
	CREATE INDEX IF NOT EXISTS idx_rides_status ON rides(status);
	CREATE INDEX IF NOT EXISTS idx_user_phone ON "user" (phone_number);
	CREATE INDEX IF NOT EXISTS idx_driver_phone ON driver (phone_number);
	CREATE INDEX IF NOT EXISTS idx_payments_rideid ON payments("rideId");

	-- Admin dashboard & filtered queries
	CREATE INDEX IF NOT EXISTS idx_rides_created ON rides("createdAt");
	CREATE INDEX IF NOT EXISTS idx_rides_vehicle ON rides("vehicleType");
	CREATE INDEX IF NOT EXISTS idx_driver_status ON driver(status);
	CREATE INDEX IF NOT EXISTS idx_rides_status_created ON rides(status, "createdAt");
	CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
	CREATE INDEX IF NOT EXISTS idx_payments_created ON payments("createdAt");

	-- Driver earnings queries (compound index)
	CREATE INDEX IF NOT EXISTS idx_rides_driver_status_created ON rides("driverId", status, "createdAt");

	-- User ride history
	CREATE INDEX IF NOT EXISTS idx_rides_user_created ON rides("userId", "createdAt");

	-- SOS & Promo
	CREATE INDEX IF NOT EXISTS idx_sos_rideid ON sos_alerts("rideId");
	CREATE INDEX IF NOT EXISTS idx_promo_code ON promo_codes(code);
	CREATE INDEX IF NOT EXISTS idx_user_status ON "user"(status);

	-- Driver online dispatch (fast lookup for ride matching)
	CREATE INDEX IF NOT EXISTS idx_driver_online_active ON driver("isOnline", status) WHERE "isOnline"=TRUE AND status='active';

	-- ═══════════════════════════════════════════
	-- EXTERNAL API LOGS TABLE — centralized audit
	-- ═══════════════════════════════════════════
	CREATE TABLE IF NOT EXISTS external_api_logs (
		id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
		provider TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		"requestId" TEXT UNIQUE,
		"requestPayload" JSONB,
		"responsePayload" JSONB,
		"statusCode" INTEGER,
		"durationMs" INTEGER,
		"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_api_logs_requestid ON external_api_logs("requestId");
	CREATE INDEX IF NOT EXISTS idx_api_logs_created ON external_api_logs("createdAt");
	`

	_, err := Pool.Exec(context.Background(), sql)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Database migration completed successfully")
}
