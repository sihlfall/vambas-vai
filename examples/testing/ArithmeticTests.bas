Attribute VB_Name = "ArithmeticTests"
Option Explicit
'@TestModule
'@Folder("Compiler tests")
Private Assert As Object
Private state As Long

'@ModuleInitialize
Private Sub StartModule()
    Set Assert = CreateObject("Rubberduck.AssertClass")
    state = 40
End Sub

'@TestInitialize
Private Sub BeforeEach()
    state = state + 1
End Sub

'@TestCleanup
Private Sub AfterEach()
    state = state - 1
End Sub

'@ModuleCleanup
Private Sub FinishModule()
    Assert.AreEqual 40&, state, "Module state survives until cleanup"
    Set Assert = Nothing
End Sub

'@TestMethod("Arithmetic")
Private Sub Addition()
    Assert.AreEqual 42&, state + 1
    Assert.IsTrue state = 41
End Sub

'@TestMethod("Arrays")
Private Sub ArrayValues()
    Dim expected(2 To 3) As Long
    Dim actual(2 To 3) As Long
    expected(2) = 7
    actual(2) = 7
    Assert.SequenceEquals expected, actual
End Sub

'@IgnoreTest
'@TestMethod("Future")
Private Sub PendingWork()
    Assert.Fail "Ignored tests are never executed"
End Sub
