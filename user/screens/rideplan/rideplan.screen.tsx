import {
  View,
  Text,
  KeyboardAvoidingView,
  Platform,
  TouchableOpacity,
  Dimensions,
  Pressable,
  ScrollView,
  Image,
  ActivityIndicator,
  TextInput,
  FlatList,
} from "react-native";
import styles from "./styles";
import { useCallback, useEffect, useRef, useState } from "react";
import { external } from "@/styles/external.style";
import { windowHeight, windowWidth } from "@/themes/app.constant";
import MapView, { Marker, Polyline, UrlTile } from "react-native-maps";
import { router } from "expo-router";
import { Clock, LeftArrow, PickLocation, PickUpLocation } from "@/utils/icons";
import color from "@/themes/app.colors";
import DownArrow from "@/assets/icons/downArrow";
import PlaceHolder from "@/assets/icons/placeHolder";
import _ from "lodash";
import axios from "axios";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { Toast } from "react-native-toast-notifications";
import moment from "moment";
import { parseDuration } from "@/utils/time/parse.duration";
import Button from "@/components/common/button";
import { useGetUserData } from "@/hooks/useGetUserData";
import * as ExpoLocation from "expo-location";
import * as Notifications from "expo-notifications";
import * as Device from "expo-device";
import Constants from "expo-constants";
import { io } from "socket.io-client";
import { decode } from "@/utils/polyline";

export default function RidePlanScreen() {
  const { user } = useGetUserData();
  const socket = useRef<any>(null);
  const notificationListener = useRef<any>();
  const [wsConnected, setWsConnected] = useState(false);
  const [places, setPlaces] = useState<any>([]);
  const [query, setQuery] = useState("");
  const [region, setRegion] = useState<any>({
    latitude: 37.78825,
    longitude: -122.4324,
    latitudeDelta: 0.0922,
    longitudeDelta: 0.0421,
  });
  const [marker, setMarker] = useState<any>(null);
  const [currentLocation, setCurrentLocation] = useState<any>(null);
  const [distance, setDistance] = useState<any>(null);
  const [locationSelected, setlocationSelected] = useState(false);
  const [selectedVehcile, setselectedVehcile] = useState("Car");
  const [travelTimes, setTravelTimes] = useState({
    driving: null,
    walking: null,
    bicycling: null,
    transit: null,
  });
  const [keyboardAvoidingHeight, setkeyboardAvoidingHeight] = useState(false);
  const [driverLists, setdriverLists] = useState([]);
  const [selectedDriver, setselectedDriver] = useState<DriverType>();
  const [driverLoader, setdriverLoader] = useState(true);
  const [routeCoordinates, setRouteCoordinates] = useState<any[]>([]);

  Notifications.setNotificationHandler({
    handleNotification: async () => ({
      shouldShowAlert: true,
      shouldPlaySound: true,
      shouldSetBadge: false,
    }),
  });

  useEffect(() => {
    notificationListener.current =
      Notifications.addNotificationReceivedListener((notification) => {
        const orderData = {
          currentLocation: notification.request.content.data.currentLocation,
          marker: notification.request.content.data.marker,
          distance: notification.request.content.data.distance,
          driver: notification.request.content.data.orderData,
        };
        router.push({
          pathname: "/(routes)/ride-details",
          params: { orderData: JSON.stringify(orderData) },
        });
      });

    return () => {
      Notifications.removeNotificationSubscription(
        notificationListener.current,
      );
    };
  }, []);

  useEffect(() => {
    (async () => {
      let { status } = await ExpoLocation.requestForegroundPermissionsAsync();
      if (status !== "granted") {
        Toast.show(
          "Please approve your location tracking otherwise you can't use this app!",
          {
            type: "danger",
            placement: "bottom",
          },
        );
      }

      let location = await ExpoLocation.getCurrentPositionAsync({
        accuracy: ExpoLocation.Accuracy.High,
      });
      const { latitude, longitude } = location.coords;
      setCurrentLocation({ latitude, longitude });
      setRegion({
        latitude,
        longitude,
        latitudeDelta: 0.0922,
        longitudeDelta: 0.0421,
      });
    })();
  }, []);

  const initializeWebSocket = () => {
    // Remove /api/v1 from the URI for socket connection
    const socketUri = process.env.EXPO_PUBLIC_SERVER_URI!.replace(
      "/api/v1",
      "",
    );

    socket.current = io(socketUri);

    socket.current.on("connect", () => {
      console.log("Connected to socket.io server");
      setWsConnected(true);
      if (user?.id) {
        socket.current.emit("joinUserRoom", { userId: user.id });
      }
    });

    socket.current.on("nearbyDrivers", async (message: any) => {
      await getDriversData(message.drivers);
    });

    socket.current.on("connect_error", (e: any) => {
      console.log("Socket connection error:", e.message);
    });

    socket.current.on("disconnect", (reason: string) => {
      console.log("Socket disconnected:", reason);
      setWsConnected(false);
    });
  };

  useEffect(() => {
    initializeWebSocket();
    return () => {
      if (socket.current) {
        socket.current.disconnect();
      }
    };
  }, []);

  useEffect(() => {
    registerForPushNotificationsAsync();
  }, []);

  async function registerForPushNotificationsAsync() {
    if (Device.isDevice) {
      const { status: existingStatus } =
        await Notifications.getPermissionsAsync();
      let finalStatus = existingStatus;
      if (existingStatus !== "granted") {
        const { status } = await Notifications.requestPermissionsAsync();
        finalStatus = status;
      }
      if (finalStatus !== "granted") {
        Toast.show("Failed to get push token for push notification!", {
          type: "danger",
        });
        return;
      }
      const projectId =
        Constants?.expoConfig?.extra?.eas?.projectId ??
        Constants?.easConfig?.projectId;
      if (!projectId) {
        Toast.show("Failed to get project id for push notification!", {
          type: "danger",
        });
      }
      try {
        const pushTokenString = (
          await Notifications.getExpoPushTokenAsync({
            projectId,
          })
        ).data;
        console.log(pushTokenString);
        // return pushTokenString;
      } catch (e: unknown) {
        Toast.show(`${e}`, {
          type: "danger",
        });
      }
    } else {
      Toast.show("Must use physical device for Push Notifications", {
        type: "danger",
      });
    }

    if (Platform.OS === "android") {
      Notifications.setNotificationChannelAsync("default", {
        name: "default",
        importance: Notifications.AndroidImportance.MAX,
        vibrationPattern: [0, 250, 250, 250],
        lightColor: "#FF231F7C",
      });
    }
  }

  const fetchPlaces = async (input: any) => {
    try {
      const response = await axios.get(
        `https://nominatim.openstreetmap.org/search?format=json&q=${input}&countrycodes=in&limit=5&addressdetails=1`,
      );
      setPlaces(response.data);
    } catch (error) {
      console.log(error);
    }
  };

  const debouncedFetchPlaces = useCallback(_.debounce(fetchPlaces, 500), []);

  useEffect(() => {
    if (query.length > 2) {
      debouncedFetchPlaces(query);
    } else {
      setPlaces([]);
    }
  }, [query, debouncedFetchPlaces]);

  const handleInputChange = (text: any) => {
    setQuery(text);
  };

  const fetchTravelTimes = async (origin: any, destination: any) => {
    // Call backend API for ride estimate (which uses Ola Maps)
    try {
      const accessToken = await AsyncStorage.getItem("accessToken");
      const response = await axios.post(
        `${process.env.EXPO_PUBLIC_SERVER_URI}/ride-estimate`,
        {
          origin: `${origin.latitude},${origin.longitude}`,
          destination: `${destination.latitude},${destination.longitude}`,
          vehicleType: selectedVehcile,
        },
        {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
        },
      );

      if (response.data.success) {
        const {
          polyline: encodedPolyline,
          distance,
          duration,
          fare,
          estimatedDuration,
        } = response.data.data;

        // Update state
        setTravelTimes({
          driving: `${estimatedDuration} mins`,
          walking: null,
          bicycling: null,
          transit: null,
        });

        setDistance(distance / 1000); // Convert meters to km for display logic if needed (or keep consistency)

        // Decode polyline
        const points = decode(encodedPolyline);
        const coords = points.map((point: any) => ({
          latitude: point[0],
          longitude: point[1],
        }));
        setRouteCoordinates(coords);
      }
    } catch (error) {
      console.log("Ride Estimate Error:", error);
      Toast.show("Failed to get ride estimate", { type: "danger" });
    }
  };

  const handlePlaceSelect = async (place: any) => {
    try {
      const lat = parseFloat(place.lat);
      const lng = parseFloat(place.lon);

      const selectedDestination = { latitude: lat, longitude: lng };
      setRegion({
        ...region,
        latitude: lat,
        longitude: lng,
      });
      setMarker({
        latitude: lat,
        longitude: lng,
      });
      setPlaces([]);
      setQuery(place.display_name); // Update input with selected place name
      requestNearbyDrivers();
      setlocationSelected(true);
      setkeyboardAvoidingHeight(false);
      if (currentLocation) {
        await fetchTravelTimes(currentLocation, selectedDestination);
      }
    } catch (error) {
      console.log(error);
    }
  };

  const calculateDistance = (lat1: any, lon1: any, lat2: any, lon2: any) => {
    var p = 0.017453292519943295; // Math.PI / 180
    var c = Math.cos;
    var a =
      0.5 -
      c((lat2 - lat1) * p) / 2 +
      (c(lat1 * p) * c(lat2 * p) * (1 - c((lon2 - lon1) * p))) / 2;

    return 12742 * Math.asin(Math.sqrt(a)); // 2 * R; R = 6371 km
  };

  const getEstimatedArrivalTime = (travelTime: any) => {
    const now = moment();
    const travelMinutes = parseDuration(travelTime);
    const arrivalTime = now.add(travelMinutes, "minutes");
    return arrivalTime.format("hh:mm A");
  };

  useEffect(() => {
    if (marker && currentLocation) {
      const dist = calculateDistance(
        currentLocation.latitude,
        currentLocation.longitude,
        marker.latitude,
        marker.longitude,
      );
      setDistance(dist);
    }
  }, [marker, currentLocation]);

  const getNearbyDrivers = () => {
    // This is now handled in initializeWebSocket via event listener
  };

  const getDriversData = async (drivers: any) => {
    // Extract driver IDs from the drivers array
    const driverIds = drivers.map((driver: any) => driver.id).join(",");
    const response = await axios.get(
      `${process.env.EXPO_PUBLIC_SERVER_URI}/driver/get-drivers-data`,
      {
        params: { ids: driverIds },
      },
    );

    const driverData = response.data;
    setdriverLists(driverData);
    setdriverLoader(false);
  };

  const requestNearbyDrivers = () => {
    if (currentLocation && wsConnected) {
      socket.current.emit("requestRide", {
        role: "user",
        latitude: currentLocation.latitude,
        longitude: currentLocation.longitude,
        userId: user?.id,
      });
    }
  };

  const sendPushNotification = async (expoPushToken: string, data: any) => {
    const message = {
      to: expoPushToken,
      sound: "default",
      title: "New Ride Request",
      body: "You have a new ride request.",
      data: { orderData: data },
    };

    await axios.post("https://exp.host/--/api/v2/push/send", message);
  };

  const handleOrder = async () => {
    // We already have location names from selection / current location reverse geocoding (if available)
    // For now, let's use the coordinates or placeholders if names are missing.
    // In a real OSM implementation, we should reverse geocode the current location using Nominatim if needed.

    // Reverse Geocode Current Location if not already known
    let currentLocName = "Current Location";
    if (currentLocation) {
      try {
        const res = await axios.get(
          `https://nominatim.openstreetmap.org/reverse?format=json&lat=${currentLocation.latitude}&lon=${currentLocation.longitude}`,
        );
        currentLocName = res.data.display_name;
      } catch (e) {
        console.log("Reverse geocode error", e);
      }
    }

    const data = {
      user,
      currentLocation,
      marker,
      distance: distance ? distance.toFixed(2) : "0",
      currentLocationName: currentLocName,
      destinationLocation: query, // Query holds the selected destination name
    };
    const driverPushToken = "ExponentPushToken[v1e34ML-hnypD7MKQDDwaK]";

    await sendPushNotification(driverPushToken, JSON.stringify(data));
  };

  return (
    <KeyboardAvoidingView
      style={[external.fx_1]}
      behavior={Platform.OS === "ios" ? "padding" : "height"}
    >
      <View>
        <View
          style={{ height: windowHeight(!keyboardAvoidingHeight ? 500 : 300) }}
        >
          <MapView
            style={{ flex: 1 }}
            region={region}
            onRegionChangeComplete={(region) => setRegion(region)}
          >
            <UrlTile
              urlTemplate="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
              maximumZ={19}
              zIndex={1}
            />
            {marker && <Marker coordinate={marker} />}
            {currentLocation && <Marker coordinate={currentLocation} />}
            {routeCoordinates.length > 0 && (
              <Polyline
                coordinates={routeCoordinates}
                strokeWidth={4}
                strokeColor="blue"
              />
            )}
          </MapView>
        </View>
      </View>
      <View style={styles.contentContainer}>
        <View style={[styles.container]}>
          {locationSelected ? (
            <>
              {driverLoader ? (
                <View
                  style={{
                    flex: 1,
                    alignItems: "center",
                    justifyContent: "center",
                    height: 400,
                  }}
                >
                  <ActivityIndicator size={"large"} />
                </View>
              ) : (
                <ScrollView
                  style={{
                    paddingBottom: windowHeight(20),
                    height: windowHeight(280),
                  }}
                >
                  <View
                    style={{
                      borderBottomWidth: 1,
                      borderBottomColor: "#b5b5b5",
                      paddingBottom: windowHeight(10),
                      flexDirection: "row",
                    }}
                  >
                    <Pressable onPress={() => setlocationSelected(false)}>
                      <LeftArrow />
                    </Pressable>
                    <Text
                      style={{
                        margin: "auto",
                        fontSize: 20,
                        fontWeight: "600",
                      }}
                    >
                      Gathering options
                    </Text>
                  </View>
                  <View style={{ padding: windowWidth(10) }}>
                    {driverLists?.map((driver: DriverType) => (
                      <Pressable
                        style={{
                          width: windowWidth(420),
                          borderWidth:
                            selectedVehcile === driver.vehicle_type ? 2 : 0,
                          borderRadius: 10,
                          padding: 10,
                          marginVertical: 5,
                        }}
                        onPress={() => {
                          setselectedVehcile(driver.vehicle_type);
                        }}
                      >
                        <View style={{ margin: "auto" }}>
                          <Image
                            source={
                              driver?.vehicle_type === "Car"
                                ? require("@/assets/images/vehicles/car.png")
                                : driver?.vehicle_type === "Motorcycle"
                                  ? require("@/assets/images/vehicles/bike.png")
                                  : require("@/assets/images/vehicles/bike.png")
                            }
                            style={{ width: 90, height: 80 }}
                          />
                        </View>
                        <View
                          style={{
                            flexDirection: "row",
                            alignItems: "center",
                            justifyContent: "space-between",
                          }}
                        >
                          <View>
                            <Text style={{ fontSize: 20, fontWeight: "600" }}>
                              RideWave {driver?.vehicle_type}
                            </Text>
                            <Text style={{ fontSize: 16 }}>
                              {getEstimatedArrivalTime(travelTimes.driving)}{" "}
                              dropoff
                            </Text>
                          </View>
                          <Text
                            style={{
                              fontSize: windowWidth(20),
                              fontWeight: "600",
                            }}
                          >
                            BDT{" "}
                            {(
                              distance.toFixed(2) * parseInt(driver.rate)
                            ).toFixed(2)}
                          </Text>
                        </View>
                      </Pressable>
                    ))}

                    <View
                      style={{
                        paddingHorizontal: windowWidth(10),
                        marginTop: windowHeight(15),
                      }}
                    >
                      <Button
                        backgroundColor={"#000"}
                        textColor="#fff"
                        title={`Confirm Booking`}
                        onPress={() => handleOrder()}
                      />
                    </View>
                  </View>
                </ScrollView>
              )}
            </>
          ) : (
            <>
              <View style={{ flexDirection: "row", alignItems: "center" }}>
                <TouchableOpacity onPress={() => router.back()}>
                  <LeftArrow />
                </TouchableOpacity>
                <Text
                  style={{
                    margin: "auto",
                    fontSize: windowWidth(25),
                    fontWeight: "600",
                  }}
                >
                  Plan your ride
                </Text>
              </View>
              {/* picking up time */}
              <View
                style={{
                  width: windowWidth(200),
                  height: windowHeight(28),
                  borderRadius: 20,
                  backgroundColor: color.lightGray,
                  alignItems: "center",
                  justifyContent: "center",
                  marginVertical: windowHeight(10),
                }}
              >
                <View style={{ flexDirection: "row", alignItems: "center" }}>
                  <Clock />
                  <Text
                    style={{
                      fontSize: windowHeight(12),
                      fontWeight: "600",
                      paddingHorizontal: 8,
                    }}
                  >
                    Pick-up now
                  </Text>
                  <DownArrow />
                </View>
              </View>
              {/* picking up location */}
              <View
                style={{
                  borderWidth: 2,
                  borderColor: "#000",
                  borderRadius: 15,
                  marginBottom: windowHeight(15),
                  paddingHorizontal: windowWidth(15),
                  paddingVertical: windowHeight(5),
                }}
              >
                <View style={{ flexDirection: "row" }}>
                  <PickLocation />
                  <View
                    style={{
                      width: Dimensions.get("window").width * 1 - 110,
                      borderBottomWidth: 1,
                      borderBottomColor: "#999",
                      marginLeft: 5,
                      height: windowHeight(20),
                    }}
                  >
                    <Text
                      style={{
                        color: "#2371F0",
                        fontSize: 18,
                        paddingLeft: 5,
                      }}
                    >
                      Current Location
                    </Text>
                  </View>
                </View>
                <View
                  style={{
                    flexDirection: "row",
                    paddingVertical: 12,
                  }}
                >
                  <PlaceHolder />
                  <View
                    style={{
                      marginLeft: 5,
                      width: Dimensions.get("window").width * 1 - 110,
                    }}
                  >
                    <TextInput
                      placeholder="Where to?"
                      placeholderTextColor="#000"
                      style={{
                        height: 38,
                        color: "#000",
                        fontSize: 16,
                        width: "100%",
                      }}
                      value={query}
                      onChangeText={handleInputChange}
                      onFocus={() => setkeyboardAvoidingHeight(true)}
                    />
                  </View>
                </View>
              </View>
              {/* Last sessions / Search Results */}
              {places.length > 0 && (
                <View style={{ maxHeight: windowHeight(200) }}>
                  <FlatList
                    data={places}
                    keyExtractor={(item: any) =>
                      item.place_id
                        ? item.place_id.toString()
                        : Math.random().toString()
                    }
                    renderItem={({ item }) => (
                      <Pressable
                        style={{
                          flexDirection: "row",
                          alignItems: "center",
                          marginBottom: windowHeight(20),
                        }}
                        onPress={() => handlePlaceSelect(item)}
                      >
                        <PickUpLocation />
                        <Text style={{ paddingLeft: 15, fontSize: 18 }}>
                          {item.display_name}
                        </Text>
                      </Pressable>
                    )}
                  />
                </View>
              )}
            </>
          )}
        </View>
      </View>
    </KeyboardAvoidingView>
  );
}
