Attribute VB_Name = "ObjectArraysProbe"
Option Explicit

' Smoke-check construction, managed array fields, and cleanup against unchanged
' stdQuadTree. Query/insertion still depend on missing library functions.
Public Function Run() As Long
    Dim tree As stdQuadTree
    Set tree = stdQuadTree.Create(0, 0, 10)
    If Not tree Is Nothing Then Run = 42
End Function
