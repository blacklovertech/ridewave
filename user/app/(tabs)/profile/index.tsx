import { View, Text } from "react-native";
import React from "react";
import { fontSizes, windowHeight, windowWidth } from "@/themes/app.constant";
import Input from "@/components/common/input";
import SelectInput from "@/components/common/select-input";
import Button from "@/components/common/button";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { router } from "expo-router";
import { useGetUserData } from "@/hooks/useGetUserData";
import axios from "axios";

export default function Profile() {
  const { user, loading } = useGetUserData();

  const [name, setName] = React.useState("");
  const [loadingUpdate, setLoadingUpdate] = React.useState(false);

  React.useEffect(() => {
    if (user) {
      setName(user.name || "");
    }
  }, [user]);

  const handleUpdate = async () => {
    setLoadingUpdate(true);
    // Use SendingOtpToEmail endpoint for profile update logic as it handles name update
    // OR create a specific profile update endpoint. The user.go has UpdateUserNotificationToken but no generic update.
    // However, SendingOtpToEmail updates name if email is empty or sends OTP.
    // Actually we should use a dedicated update endpoint or reuse SendingOtpToEmail with empty email to just update name?
    // Let's use SendingOtpToEmail with empty email to update name if email is unchanged/empty.
    // But SendingOtpToEmail requires userId.

    // Better: Creating a new endpoint is safer, but avoiding server changes if possible.
    // Re-using SendingOtpToEmail:
    // If email is empty, it updates name.
    await axios
      .post(`${process.env.EXPO_PUBLIC_SERVER_URI}/email-otp-request`, {
        email: "", // Keep email empty to just update name and get token back
        name: name,
        userId: user.id,
      })
      .then((res) => {
        setLoadingUpdate(false);
        // Refresh user data?
        alert("Profile updated!");
      })
      .catch((err) => {
        setLoadingUpdate(false);
        console.log(err);
      });
  };

  if (loading) {
    return (
      <View style={{ flex: 1, justifyContent: "center", alignItems: "center" }}>
        <Text>Loading...</Text>
      </View>
    );
  }

  return (
    <View style={{ paddingTop: 70 }}>
      <Text
        style={{
          textAlign: "center",
          fontSize: fontSizes.FONT30,
          fontWeight: "600",
        }}
      >
        My Profile
      </Text>
      <View style={{ padding: windowWidth(20) }}>
        <Input
          title="Name"
          value={name}
          onChangeText={setName}
          placeholder="Enter your name"
        />
        <Input
          title="Email Address"
          value={user?.email}
          onChangeText={() => {}}
          placeholder={user?.email || "No Email"}
          disabled={true}
        />
        <Input
          title="Phone Number"
          value={user?.phone_number}
          onChangeText={() => {}}
          placeholder={user?.phone_number || ""}
          disabled={true}
        />
        <View style={{ marginVertical: 25 }}>
          <Button
            onPress={handleUpdate}
            title="Update Profile"
            loading={loadingUpdate}
            disabled={loadingUpdate}
          />
          <View style={{ marginTop: 15 }}>
            <Button
              onPress={async () => {
                await AsyncStorage.removeItem("accessToken");
                router.push("/(routes)/login");
              }}
              title="Log Out"
              backgroundColor="crimson"
            />
          </View>
        </View>
      </View>
    </View>
  );
}
