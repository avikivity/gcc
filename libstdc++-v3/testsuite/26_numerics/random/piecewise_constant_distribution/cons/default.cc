// { dg-do run { target c++11 } }
// { dg-require-cstdint "" }
//
// 2008-12-03  Edward M. Smith-Rowland <3dw4rd@verizon.net>
//
// Copyright (C) 2008-2016 Free Software Foundation, Inc.
//
// This file is part of the GNU ISO C++ Library.  This library is free
// software; you can redistribute it and/or modify it under the
// terms of the GNU General Public License as published by the
// Free Software Foundation; either version 3, or (at your option)
// any later version.
//
// This library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this library; see the file COPYING3.  If not see
// <http://www.gnu.org/licenses/>.

// 26.4.8.5.2 Class template piecewise_constant_distribution [rand.dist.samp.pconst]
// 26.4.2.4 Concept RandomNumberDistribution [rand.concept.dist]

#include <random>
#include <testsuite_hooks.h>

void
test01()
{
  bool test __attribute__((unused)) = true;

  std::piecewise_constant_distribution<> u;
  std::vector<double> interval = u.intervals();
  std::vector<double> density = u.densities();
  VERIFY( interval.size() == 2 );
  VERIFY( interval[0] == 0.0 );
  VERIFY( interval[1] == 1.0 );
  VERIFY( density.size() == 1 );
  VERIFY( density[0] == 1.0 );
}

int main()
{
  test01();
  return 0;
}
