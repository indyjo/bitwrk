# ##### BEGIN GPL LICENSE BLOCK #####
#
#  BitWrk - A Bitcoin-friendly, anonymous marketplace for computing power
#  Copyright (C) 2013-2016  Jonas Eschenburg <jonas@bitwrk.net>
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, either version 3 of the License, or
#  (at your option) any later version.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License
#  along with this program.  If not, see <http://www.gnu.org/licenses/>.
#
# ##### END GPL LICENSE BLOCK #####

import math

""" Calculates an optimal regular tiling for a BitWrk render of specified resolution.
    A regular tiling of a surface of dimensions WxH is specified by numbers u,v > 0
    denoting the number of tiles along the X and Y axis, respectively. An optimal
    tiling minimizes the sum of edge lengths, H*(u+1) + W*(v+1), and thereby also
    minimizes Hu+Wv.
    A tiling is feasible if the largest tile has area w*h <= C, with
    w = ceil(W/u) and h = ceil(H/v)
"""
def optimal_tiling(W, H, C):
    # Starting with an edge length <= sqrt(C) guarantees a feasible initial value.
    L = math.floor(math.sqrt(C))
    # ceil(W/u) and ceil(H/v) must both be <= L
    uv = (int(math.ceil(W / L)), int(math.ceil(H / L)))
    def is_feasible(uv):
        u, v = uv
        return u > 0 and v > 0 and math.ceil(W / u) * math.ceil(H / v) <= C
    if H > W:
        def walk(uv):
            u, v = uv
            yield (u - 1, v)
            yield (u, v - 1)
    else:
        def walk(uv):
            u, v = uv
            yield (u, v - 1)
            yield (u - 1, v)
            
    if not is_feasible(uv):
        raise RuntimeError(uv)
        
    found = True
    while found:
        # print("Evaluating", uv)
        found = False
        for candidate in walk(uv):
            if is_feasible(candidate):
                found = True
                uv = candidate
                break
    return uv

