import { View, Text, Image, TouchableOpacity, Alert } from "react-native";
import React, { useState } from "react";
import AuthContainer from "@/utils/container/auth-container";
import { windowHeight } from "@/themes/app.constant";
import styles from "./styles";
import Images from "@/utils/images";
import SignInText from "@/components/login/signin.text";
import { external } from "@/styles/external.style";
import PhoneNumberInput from "@/components/login/phone-number.input";
import Button from "@/components/common/button";
import { router } from "expo-router";
import { useToast } from "react-native-toast-notifications";
import axios from "axios";

export default function LoginScreen() {
  const [phone_number, setphone_number] = useState("");
  const [loading, setloading] = useState(false);
  const [countryCode, setCountryCode] = useState("+880");
  const toast = useToast();

  const handleSubmit = async () => {
    if (phone_number === "" || countryCode === "") {
      if (toast) {
        toast.show("Please fill the fields!", {
          placement: "bottom",
        });
      }
    } else {
      if (!phone_number || phone_number.length !== 10) {
        if (toast) {
          toast.show("Please enter a valid 10-digit phone number!", {
            placement: "bottom",
            type: "danger",
          });
        } else {
          Alert.alert("Error", "Please enter a valid 10-digit phone number!");
        }
        return;
      }
      setloading(true);
      const phoneNumber = `${countryCode}${phone_number}`;
      console.log("Sending Phone Number:", phoneNumber);
      await axios
        .post(`${process.env.EXPO_PUBLIC_SERVER_URI}/registration`, {
          phone_number: phoneNumber,
        })
        .then((res) => {
          setloading(false);
          router.push({
            pathname: "/(routes)/otp-verification",
            params: { phoneNumber },
          });
        })
        .catch((error) => {
          console.log(error);
          setloading(false);
          if (toast) {
            toast.show(
              "Something went wrong! please re check your phone number!",
              {
                type: "danger",
                placement: "bottom",
              },
            );
          }
        });
    }
  };
  return (
    <AuthContainer
      topSpace={windowHeight(150)}
      imageShow={true}
      container={
        <View>
          <View>
            <View>
              <Image style={styles.transformLine} source={Images.line} />
              <SignInText />
              <View style={[external.mt_25, external.Pb_10]}>
                <PhoneNumberInput
                  phone_number={phone_number}
                  setphone_number={setphone_number}
                  countryCode={countryCode}
                  setCountryCode={setCountryCode}
                />
                <View style={[external.mt_25, external.Pb_15]}>
                  <Button
                    title="Get Otp"
                    onPress={() => handleSubmit()}
                    disabled={loading}
                  />
                </View>
                <View style={{ alignItems: "center", marginBottom: 20 }}>
                  <Text
                    style={{
                      fontFamily: "TT-Octosquares-Medium",
                      fontSize: 16,
                      marginBottom: 10,
                    }}
                  >
                    Or sign in with
                  </Text>
                  <TouchableOpacity
                    style={{
                      flexDirection: "row",
                      alignItems: "center",
                      justifyContent: "center",
                      backgroundColor: "#fff",
                      paddingVertical: 10,
                      paddingHorizontal: 20,
                      borderRadius: 5,
                      borderWidth: 1,
                      borderColor: "#ddd",
                      width: "100%",
                    }}
                    onPress={() => {
                      // Google Sign-In Logic Placeholder
                      if (toast)
                        toast.show("Google Sign-In coming soon!", {
                          placement: "bottom",
                        });
                    }}
                  >
                    <Image
                      source={{
                        uri: "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c1/Google_%22G%22_logo.svg/1024px-Google_%22G%22_logo.svg.png",
                      }}
                      style={{ width: 24, height: 24, marginRight: 10 }}
                      resizeMode="contain"
                    />
                    <Text
                      style={{ fontSize: 16, fontWeight: "600", color: "#333" }}
                    >
                      Google
                    </Text>
                  </TouchableOpacity>
                </View>
              </View>
            </View>
          </View>
        </View>
      }
    />
  );
}
