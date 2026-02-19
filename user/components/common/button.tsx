import { Pressable, StyleSheet, Text, ActivityIndicator } from "react-native";
import React from "react";
import { commonStyles } from "@/styles/common.style";
import color from "@/themes/app.colors";
import { windowHeight } from "@/themes/app.constant";
import { external } from "@/styles/external.style";

interface ButtonProps {
  title: string;
  onPress: () => void;
  width?: number | string;
  backgroundColor?: string;
  textColor?: string;
  disabled?: boolean;
  loading?: boolean;
}

const Button: React.FC<ButtonProps> = ({
  title,
  onPress,
  width,
  backgroundColor,
  textColor,
  disabled,
  loading,
}) => {
  const widthNumber = width || "100%";
  return (
    <Pressable
      style={[
        styles.container,
        {
          width: widthNumber,
          backgroundColor: backgroundColor || color.buttonBg,
          opacity: disabled || loading ? 0.7 : 1,
        },
      ]}
      onPress={!loading ? onPress : undefined}
      disabled={disabled || loading}
    >
      {loading ? (
        <ActivityIndicator size="small" color={textColor || color.whiteColor} />
      ) : (
        <Text
          style={[
            commonStyles.extraBold,
            { color: textColor || color.whiteColor },
          ]}
        >
          {title}
        </Text>
      )}
    </Pressable>
  );
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: color.buttonBg,
    height: windowHeight(40),
    borderRadius: 6,
    ...external.ai_center,
    ...external.js_center,
  },
});

export default Button;
