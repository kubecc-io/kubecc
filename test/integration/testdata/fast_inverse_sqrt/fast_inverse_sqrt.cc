auto Q_rsqrt(float number) {
  auto threehalfs = 1.5F;
  auto x2 = number * 0.5F;
  auto y  = number;
  auto i  = *(long*)&y;
  i = 0x5f3759df - (i >> 1);
  y = *(float*) &i;
  y = y * (threehalfs - (x2 * y * y));
  return y;
}