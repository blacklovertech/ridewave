import { View, Text, TextInput } from "react-native";
import { commonStyles } from "@/styles/common.style";
import { windowHeight, windowWidth } from "@/themes/app.constant";
import { external } from "@/styles/external.style";
import styles from "@/screens/login/styles";
import color from "@/themes/app.colors";
import CountryPicker, { CountryCode } from "react-native-country-picker-modal";

interface Props {
  width?: number;
  phone_number: string;
  setphone_number: (phone_number: string) => void;
  countryCode: string;
  setCountryCode: (countryCode: string) => void;
}

export default function PhoneNumberInput({
  width,
  phone_number,
  setphone_number,
  countryCode,
  setCountryCode,
}: Props) {
  return (
    <View>
      <Text
        style={[
          commonStyles.mediumTextBlack,
          { marginTop: windowHeight(8), marginBottom: 8 },
        ]}
      >
        Phone Number
      </Text>
      <View
        style={[
          external.fd_row,
          external.ai_center,
          external.mt_5,
          { flexDirection: "row" },
        ]}
      >
        <View
          style={[
            styles.countryCodeContainer,
            {
              borderColor: color.border,
              justifyContent: "center",
              alignItems: "center",
              flexDirection: "row", // Ensure flag and text are in a row
              paddingHorizontal: 8, // Add padding for better touch area
            },
          ]}
        >
          <CountryPicker
            withFilter
            withFlag
            withCallingCode
            withEmoji
            countryCode={"IN"} // Default to India
            onSelect={(country) => {
              setCountryCode(`+${country.callingCode[0]}`);
            }}
            visible={false}
          />
          <Text style={[commonStyles.regularText, { marginLeft: 5 }]}>
            {countryCode || "+91"}
          </Text>
        </View>
        <View
          style={[
            styles.phoneNumberInput,
            {
              width: width || windowWidth(346),
              borderColor: color.border,
            },
          ]}
        >
          <TextInput
            style={[commonStyles.regularText]}
            placeholderTextColor={color.subtitle}
            placeholder={"Enter your number"}
            keyboardType="numeric"
            value={phone_number}
            onChangeText={setphone_number}
          />
        </View>
      </View>
    </View>
  );
}
