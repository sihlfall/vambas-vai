Attribute VB_Name = "Probe"
Option Explicit

Public Function Run() As Long
    Dim r As StaticRegex.RegexTy, pattern As String, text As String
    pattern = "(ab)+c": text = "xxababczz"
    If Not StaticRegex.TryInitializeRegex(r, pattern) Then Exit Function
    If Not StaticRegex.Test(r, text) Then Exit Function
    text = "no match"
    If StaticRegex.Test(r, text) Then Exit Function
    pattern = "("
    If StaticRegex.TryInitializeRegex(r, pattern) Then Exit Function
    Run = 42
End Function
