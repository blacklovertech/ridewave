import { View, Text, ScrollView, StyleSheet } from "react-native";
import React, { useState } from "react";
import { useTheme } from "@react-navigation/native";
import { windowHeight, windowWidth } from "@/themes/app.constant";
import TitleView from "@/components/signup/title.view";
import Input from "@/components/common/input";
import Button from "@/components/common/button";
import color from "@/themes/app.colors";
import { router, useLocalSearchParams } from "expo-router";
import axios from "axios";
import AsyncStorage from "@react-native-async-storage/async-storage";

export default function RegistrationScreen() {
  const { colors } = useTheme();
  const { user } = useLocalSearchParams() as any;
  const parsedUser = JSON.parse(user);
  const [emailFormatWarning, setEmailFormatWarning] = useState("");
  const [showWarning, setShowWarning] = useState(false);
  const [formData, setFormData] = useState({
    name: "",
    phoneNumber: "",
    email: "",
  });
  const [loading, setLoading] = useState(false);

  const handleChange = (key: string, value: string) => {
    setFormData((prevData) => ({
      ...prevData,
      [key]: value,
    }));
  };

  const handleSubmit = async () => {
    setLoading(true);
    await axios
      .post(`${process.env.EXPO_PUBLIC_SERVER_URI}/email-otp-request`, {
        email: formData.email,
        name: formData.name,
        userId: parsedUser.id,
      })
      .then(async (res) => {
        setLoading(false);
        if (formData.email === "") {
          // Email is optional, so we are done. Server sent accessToken.
          await AsyncStorage.setItem("accessToken", res.data.accessToken);
          router.push("/(tabs)/home");
        } else {
          const userData: any = {
            id: parsedUser.id,
            name: formData.name,
            email: formData.email,
            phone_number: parsedUser.phone_number,
            token: res.data.token,
          };
          router.push({
            pathname: "/(routes)/email-verification",
            params: { user: JSON.stringify(userData) },
          });
        }
      })
      .catch((error) => {
        setLoading(false);
        console.log(error);
      });
  };

  return (
    <ScrollView>
      <View>
        {/* logo */}
        <Text
          style={{
            fontFamily: "TT-Octosquares-Medium",
            fontSize: windowHeight(25),
            paddingTop: windowHeight(50),
            textAlign: "center",
          }}
        >
          Ride Wave
        </Text>
        <View style={{ padding: windowWidth(20) }}>
          <View
            style={[styles.subView, { backgroundColor: colors.background }]}
          >
            <View style={styles.space}>
              <TitleView
                title={"Create your account"}
                subTitle="Explore your life by joining Ride Wave"
              />
              <Input
                title="Name"
                placeholder="Enter your name"
                value={formData?.name}
                onChangeText={(text) => handleChange("name", text)}
                showWarning={showWarning && formData.name === ""}
                warning={"Please enter your name!"}
              />

              <Input
                title="Email Address (Optional)"
                placeholder="Enter your email address"
                keyboardType="email-address"
                value={formData.email}
                onChangeText={(text) => {
                  handleChange("email", text);
                  if (text.length > 0 && !/\S+@\S+\.\S+/.test(text)) {
                    setEmailFormatWarning("Please enter a valid email!");
                  } else {
                    setEmailFormatWarning("");
                  }
                }}
                showWarning={
                  (showWarning && formData.name === "") ||
                  emailFormatWarning !== ""
                }
                warning={
                  emailFormatWarning !== ""
                    ? emailFormatWarning
                    : "Please enter your name!"
                }
                emailFormatWarning={emailFormatWarning}
              />
              <View style={styles.margin}>
                <Button
                  onPress={() => handleSubmit()}
                  title="Next"
                  disabled={loading}
                  backgroundColor={color.buttonBg}
                  textColor={color.whiteColor}
                />
              </View>
            </View>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  main: {
    flex: 1,
  },
  subView: {
    height: "100%",
  },
  space: {
    marginHorizontal: windowWidth(4),
  },
  margin: {
    marginVertical: windowHeight(12),
  },
});
