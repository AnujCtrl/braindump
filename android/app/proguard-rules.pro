# Keep kotlinx-serialization @Serializable types intact across R8.
-keepclassmembers @kotlinx.serialization.Serializable class ** {
    *** Companion;
}
-keepclasseswithmembers class ** {
    kotlinx.serialization.KSerializer serializer(...);
}
-keepattributes *Annotation*, InnerClasses

# Retrofit needs reflection on @Serializable model classes
-keep class com.braindump.android.net.** { *; }
-keep class com.braindump.android.data.** { *; }
