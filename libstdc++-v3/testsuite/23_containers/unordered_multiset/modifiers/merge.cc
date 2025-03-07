// Copyright (C) 2016 Free Software Foundation, Inc.
//
// This file is part of the GNU ISO C++ Library.  This library is free
// software; you can redistribute it and/or modify it under the
// terms of the GNU General Public License as published by the
// Free Software Foundation; either version 3, or (at your option)
// any later version.

// This library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License along
// with this library; see the file COPYING3.  If not see
// <http://www.gnu.org/licenses/>.

// { dg-options "-std=gnu++17" }

#include <unordered_set>
#include <algorithm>
#include <testsuite_hooks.h>

using test_type = std::unordered_multiset<int>;
struct hash {
  auto operator()(int i) const noexcept { return ~std::hash<int>()(i); }
};
struct equal : std::equal_to<> { };

void
test01()
{
  bool test __attribute__((unused)) = true;

  test_type c0{ 1, 1, 2, 2, 3, 3 };
  test_type c1 = c0, c2 = c0;

  c1.merge(c2);
  for (auto i : c1)
    VERIFY( c1.count(i) == (2 * c0.count(i)) );
  VERIFY( c2.empty() );

  c1.clear();
  c2 = c0;
  c1.merge(std::move(c2));
  VERIFY( c1 == c0 );
  VERIFY( c2.empty() );
}

void
test02()
{
  bool test __attribute__((unused)) = true;

  test_type c0{ 1, 1, 2, 2, 3, 3 };
  test_type c1 = c0;
  std::unordered_multiset<int, hash, equal> c2( c0.begin(), c0.end() );

  c1.merge(c2);
  VERIFY( c1.size() == (2 * c0.size()) );
  for (auto i : c1)
    VERIFY( c1.count(i) == (2 * c0.count(i)) );
  VERIFY( c2.empty() );

  c1.clear();
  c2.insert( c0.begin(), c0.end() );
  c1.merge(std::move(c2));
  VERIFY( c1 == c0 );
  VERIFY( c2.empty() );
}

void
test03()
{
  const test_type c0{ 1, 1, 2, 2, 3, 3 };
  test_type c1 = c0;
  std::unordered_multiset<int, hash, equal> c2( c0.begin(), c0.end() );

  c1.merge(c2);
  VERIFY( c1.size() == (2 * c0.size()) );
  for (auto i : c1)
    VERIFY( c1.count(i) == (2 * c0.count(i)) );
  VERIFY( c2.empty() );

  c1.clear();
  c2.insert( c0.begin(), c0.end() );
  c1.merge(std::move(c2));
  VERIFY( c1 == c0 );
  VERIFY( c2.empty() );
}

void
test04()
{
  bool test __attribute__((unused)) = true;

  const test_type c0{ 1, 1, 2, 2, 3, 3 };
  test_type c1 = c0;
  std::unordered_set<int, hash, equal> c2( c0.begin(), c0.end() );

  c1.merge(c2);
  VERIFY( c1.size() == (1.5 * c0.size()) );
  for (auto& i : c1)
    VERIFY( c1.count(i) == (1.5 * c0.count(i)) );
  VERIFY( c2.empty() );

  c1.clear();
  c2.insert( c0.begin(), c0.end() );
  c1.merge(std::move(c2));
  VERIFY( c1.size() == (0.5 * c0.size()) );
  VERIFY( c2.empty() );
}

int
main()
{
  test01();
  test02();
  test03();
  test04();
}
