// { dg-do compile { target c++11 } }

// Copyright (C) 2013-2016 Free Software Foundation, Inc.
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

#include <type_traits>
#include <testsuite_tr1.h>

void test01()
{
  using std::remove_const;
  using std::is_same;
  using namespace __gnu_test;

  static_assert(is_same<remove_const<const volatile int>::type,
		volatile int>::value, "");
  static_assert(is_same<remove_const<const int*>::type, const int*>::value, "");
  static_assert(is_same<remove_const<const volatile ClassType>::type,
		volatile ClassType>::value, "");
  static_assert(is_same<remove_const<const ClassType*>::type,
		const ClassType*>::value, "");
}
